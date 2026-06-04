package utils

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

var (
	MongoClient *mongo.Client
	MongoDB     *mongo.Database
	Logger      *zap.SugaredLogger
)

func InitMongo(uri, database string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	clientOpts := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err = client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	MongoClient = client
	MongoDB = client.Database(database)

	// Run migrations before creating indexes
	if err := migrateResultHashes(ctx); err != nil {
		Logger.Warnf("Result hash migration warning: %v", err)
	}

	if err := createIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

func createIndexes(ctx context.Context) error {
	indexes := map[string][]mongo.IndexModel{
		"nodes": {
			{Keys: bson.D{{Key: "key", Value: 1}}, Options: options.Index().SetUnique(true)},
			{Keys: bson.D{{Key: "status", Value: 1}}},
		},
		"tasks": {
			{Keys: bson.D{{Key: "spider_id", Value: 1}}},
			{Keys: bson.D{{Key: "node_id", Value: 1}}},
			{Keys: bson.D{{Key: "status", Value: 1}}},
			{Keys: bson.D{{Key: "created_at", Value: -1}}},
		},
		"spiders": {
			{Keys: bson.D{{Key: "name", Value: 1}}},
		},
		"schedules": {
			{Keys: bson.D{{Key: "spider_id", Value: 1}}},
			{Keys: bson.D{{Key: "enabled", Value: 1}}},
		},
		"users": {
			{Keys: bson.D{{Key: "username", Value: 1}}, Options: options.Index().SetUnique(true)},
		},
		"task_logs": {
			{Keys: bson.D{{Key: "task_id", Value: 1}, {Key: "timestamp", Value: 1}}},
		},
		"system_logs": {
			{Keys: bson.D{{Key: "timestamp", Value: -1}}},
			{Keys: bson.D{{Key: "level", Value: 1}}},
		},
		"results": {
			{Keys: bson.D{{Key: "task_id", Value: 1}, {Key: "timestamp", Value: 1}}},
			{Keys: bson.D{{Key: "spider_id", Value: 1}}},
			{Keys: bson.D{{Key: "published_at", Value: -1}, {Key: "timestamp", Value: -1}}},
			{
				Keys: bson.D{{Key: "spider_id", Value: 1}, {Key: "item_hash", Value: 1}},
				Options: options.Index().SetUnique(true).SetPartialFilterExpression(
					bson.D{{Key: "item_hash", Value: bson.D{{Key: "$exists", Value: true}, {Key: "$gt", Value: ""}}}},
				),
			},
		},
		"posts": {
			{
				Keys:    bson.D{{Key: "platform", Value: 1}, {Key: "platform_id", Value: 1}},
				Options: options.Index().SetUnique(true),
			},
			{Keys: bson.D{{Key: "spider_ids", Value: 1}}},
			{Keys: bson.D{{Key: "published_at", Value: -1}, {Key: "updated_at", Value: -1}}},
			{Keys: bson.D{{Key: "task_id", Value: 1}}},
			{Keys: bson.D{{Key: "type", Value: 1}}},
		},
	}

	indexes["xueqiu_cookies"] = []mongo.IndexModel{
		{Keys: bson.D{{Key: "xueqiu_uid", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "status", Value: 1}}},
		{Keys: bson.D{{Key: "last_used_at", Value: 1}}},
	}

	indexes["target_accounts"] = []mongo.IndexModel{
		{Keys: bson.D{{Key: "platform", Value: 1}, {Key: "user_id", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "platform", Value: 1}}},
	}

	for coll, idxModels := range indexes {
		collection := MongoDB.Collection(coll)
		_, err := collection.Indexes().CreateMany(ctx, idxModels)
		if err != nil {
			return fmt.Errorf("failed to create indexes for %s: %w", coll, err)
		}
	}

	return nil
}

// migrateResultHashes backfills item_hash for existing results and removes duplicates.
func migrateResultHashes(ctx context.Context) error {
	col := MongoDB.Collection("results")

	// Drop the old broken sparse index if it exists (from previous version)
	_, _ = col.Indexes().DropOne(ctx, "spider_id_1_item_hash_1")

	// Find results missing item_hash
	filter := bson.M{"$or": bson.A{
		bson.M{"item_hash": bson.M{"$exists": false}},
		bson.M{"item_hash": ""},
	}}
	count, err := col.CountDocuments(ctx, filter)
	if err != nil || count == 0 {
		return err
	}

	Logger.Infof("Migrating %d results: backfilling item_hash...", count)

	cursor, err := col.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("migration find failed: %w", err)
	}
	defer cursor.Close(ctx)

	// Track seen hashes to detect duplicates: key = "spiderId:hash"
	seen := make(map[string]primitive.ObjectID)
	var duplicateIds []primitive.ObjectID
	var updated int64

	for cursor.Next(ctx) {
		var doc struct {
			Id       primitive.ObjectID     `bson:"_id"`
			SpiderId primitive.ObjectID     `bson:"spider_id"`
			Item     map[string]interface{} `bson:"item"`
		}
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		hash := computeResultItemHash(doc.Item)
		compositeKey := doc.SpiderId.Hex() + ":" + hash

		if existingId, exists := seen[compositeKey]; exists {
			// Duplicate found — mark the older one for deletion (keep current)
			duplicateIds = append(duplicateIds, existingId)
			seen[compositeKey] = doc.Id
		} else {
			seen[compositeKey] = doc.Id
		}

		// Update this document with its hash
		_, err := col.UpdateByID(ctx, doc.Id, bson.M{"$set": bson.M{"item_hash": hash}})
		if err == nil {
			updated++
		}
	}

	// Remove duplicates (keeping the latest one per composite key)
	if len(duplicateIds) > 0 {
		res, err := col.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": duplicateIds}})
		if err == nil {
			Logger.Infof("Migration: removed %d duplicate results", res.DeletedCount)
		}
	}

	Logger.Infof("Migration complete: backfilled %d results", updated)
	return nil
}

// computeResultItemHash generates a dedup key for a result item (same logic as grpc/server.go).
func computeResultItemHash(item map[string]interface{}) string {
	// Posts: use Weibo post ID
	if id, ok := item["id"]; ok {
		idStr := fmt.Sprintf("%v", id)
		if idStr != "" {
			return "id:" + idStr
		}
	}
	// Topics: use title as stable dedup key
	if title, ok := item["title"]; ok {
		titleStr := fmt.Sprintf("%v", title)
		if titleStr != "" {
			return "title:" + titleStr
		}
	}
	keys := make([]string, 0, len(item))
	for k := range item {
		if k == "task_id" || k == "crawled_at" || k == "rank" || k == "hot_value" || k == "post_count" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		v, _ := json.Marshal(item[k])
		h.Write([]byte(k))
		h.Write([]byte("="))
		h.Write(v)
		h.Write([]byte(";"))
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

func InitLogger() {
	logger, _ := zap.NewProduction()
	Logger = logger.Sugar()
}

func GetCollection(name string) *mongo.Collection {
	return MongoDB.Collection(name)
}

func ObjectIDFromHex(hex string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(hex)
}

// WriteSystemLog inserts a log entry into the system_logs collection.
func WriteSystemLog(level, module, message, detail string) {
	col := GetCollection("system_logs")
	_, _ = col.InsertOne(context.Background(), bson.M{
		"level":     level,
		"module":    module,
		"message":   message,
		"detail":    detail,
		"timestamp": time.Now(),
	})
}
