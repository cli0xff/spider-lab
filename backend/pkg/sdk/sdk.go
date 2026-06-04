package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type Client struct {
	taskId   string
	spiderId string
	apiURL   string
}

func NewClient() *Client {
	return &Client{
		taskId:   os.Getenv("SPIDER_LAB_TASK_ID"),
		spiderId: os.Getenv("SPIDER_LAB_SPIDER_ID"),
		apiURL:   os.Getenv("SPIDER_LAB_API_URL"),
	}
}

func (c *Client) GetTaskId() string   { return c.taskId }
func (c *Client) GetSpiderId() string { return c.spiderId }

func (c *Client) SaveItem(item interface{}) error {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/api/results/%s", c.apiURL, c.taskId),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to save item: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("save failed with status: %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) SaveItems(items []interface{}) error {
	data, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("failed to marshal items: %w", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/api/results/%s/batch", c.apiURL, c.taskId),
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("failed to save items: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
