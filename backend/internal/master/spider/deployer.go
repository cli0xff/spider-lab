package spider

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/crawlab-team/spider-lab/internal/models"
	"github.com/crawlab-team/spider-lab/internal/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Deployer struct {
	spiderDir string
}

func NewDeployer(spiderDir string) *Deployer {
	os.MkdirAll(spiderDir, 0755)
	return &Deployer{spiderDir: spiderDir}
}

func (d *Deployer) GetSpiderDir(spiderId string) string {
	return filepath.Join(d.spiderDir, spiderId)
}

func (d *Deployer) CreateSpider(ctx context.Context, spider *models.Spider) error {
	col := utils.GetCollection("spiders")
	now := time.Now()
	spider.CreatedAt = now
	spider.UpdatedAt = now

	result, err := col.InsertOne(ctx, spider)
	if err != nil {
		return fmt.Errorf("failed to create spider: %w", err)
	}
	spider.Id = result.InsertedID.(primitive.ObjectID)

	spiderDir := d.GetSpiderDir(spider.Id.Hex())
	if err := os.MkdirAll(spiderDir, 0755); err != nil {
		return fmt.Errorf("failed to create spider directory: %w", err)
	}

	// Generate template files for template-based spiders
	if spider.Type == "template" && spider.TemplateId != "" {
		if tmpl := GetTemplate(spider.TemplateId); tmpl != nil {
			for filename, content := range tmpl.Files {
				filePath := filepath.Join(spiderDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					return fmt.Errorf("failed to write template file %s: %w", filename, err)
				}
			}
			// Auto-set the command from template
			if spider.Cmd == "" {
				spider.Cmd = tmpl.Cmd
				col.UpdateOne(ctx, bson.M{"_id": spider.Id}, bson.M{"$set": bson.M{"cmd": spider.Cmd}})
			}
			// Write config.json from spider config
			if spider.Config != nil {
				configBytes, err := RenderConfig(spider.Config)
				if err != nil {
					return fmt.Errorf("failed to render config: %w", err)
				}
				configPath := filepath.Join(spiderDir, "config.json")
				if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
					return fmt.Errorf("failed to write config.json: %w", err)
				}
			}
		}
	}

	return nil
}

// EnsureTemplateFiles regenerates template files for a spider if main.py
// is missing. This handles the case where /tmp was cleaned or the template
// was updated after the spider was created.
func (d *Deployer) EnsureTemplateFiles(ctx context.Context, spider *models.Spider) error {
	if spider.Type != "template" || spider.TemplateId == "" {
		return nil
	}
	spiderDir := d.GetSpiderDir(spider.Id.Hex())
	mainPy := filepath.Join(spiderDir, "main.py")
	if _, err := os.Stat(mainPy); err == nil {
		return nil // main.py exists, nothing to do
	}

	tmpl := GetTemplate(spider.TemplateId)
	if tmpl == nil {
		return fmt.Errorf("template %s not found", spider.TemplateId)
	}

	if err := os.MkdirAll(spiderDir, 0755); err != nil {
		return fmt.Errorf("failed to create spider directory: %w", err)
	}

	for filename, content := range tmpl.Files {
		filePath := filepath.Join(spiderDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write template file %s: %w", filename, err)
		}
	}

	// Rewrite config.json from spider config
	if spider.Config != nil {
		configBytes, err := RenderConfig(spider.Config)
		if err != nil {
			return fmt.Errorf("failed to render config: %w", err)
		}
		configPath := filepath.Join(spiderDir, "config.json")
		if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
			return fmt.Errorf("failed to write config.json: %w", err)
		}
	}

	utils.Logger.Infof("Regenerated template files for spider %s (template: %s)", spider.Id.Hex(), spider.TemplateId)
	return nil
}

func (d *Deployer) UploadFile(spiderId, filename string, content io.Reader) error {
	spiderDir := d.GetSpiderDir(spiderId)
	if err := os.MkdirAll(spiderDir, 0755); err != nil {
		return err
	}

	filePath := filepath.Join(spiderDir, filename)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, content)
	return err
}

func (d *Deployer) ListFiles(spiderId string) ([]FileInfo, error) {
	spiderDir := d.GetSpiderDir(spiderId)
	var files []FileInfo

	err := filepath.Walk(spiderDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(spiderDir, path)
		if relPath == "." {
			return nil
		}
		files = append(files, FileInfo{
			Name:    info.Name(),
			Path:    relPath,
			Size:    info.Size(),
			IsDir:   info.IsDir(),
			ModTime: info.ModTime(),
		})
		return nil
	})

	return files, err
}

func (d *Deployer) ReadFile(spiderId, relPath string) ([]byte, error) {
	filePath := filepath.Join(d.GetSpiderDir(spiderId), filepath.Clean(relPath))
	return os.ReadFile(filePath)
}

func (d *Deployer) WriteFile(spiderId, relPath string, content []byte) error {
	spiderDir := d.GetSpiderDir(spiderId)
	filePath := filepath.Join(spiderDir, filepath.Clean(relPath))
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, content, 0644)
}

func (d *Deployer) UpdateConfig(spiderId string, config map[string]interface{}) error {
	configBytes, err := RenderConfig(config)
	if err != nil {
		return fmt.Errorf("failed to render config: %w", err)
	}
	return d.WriteFile(spiderId, "config.json", configBytes)
}

func (d *Deployer) DeleteSpider(ctx context.Context, spiderId string) error {
	id, err := primitive.ObjectIDFromHex(spiderId)
	if err != nil {
		return err
	}

	col := utils.GetCollection("spiders")
	_, err = col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}

	spiderDir := d.GetSpiderDir(spiderId)
	return os.RemoveAll(spiderDir)
}

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
}
