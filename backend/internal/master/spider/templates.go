package spider

import "encoding/json"

// ConfigField defines a user-configurable parameter for a spider template.
type ConfigField struct {
	Key          string      `json:"key"`
	Label        string      `json:"label"`
	Type         string      `json:"type"` // string, number, boolean, select
	Default      interface{} `json:"default"`
	Required     bool        `json:"required"`
	Description  string      `json:"description"`
	Options      []string    `json:"options,omitempty"`
	OptionLabels []string    `json:"option_labels,omitempty"` // Human-readable labels for select options
	Min          *float64    `json:"min,omitempty"`
	Max          *float64    `json:"max,omitempty"`
	Placeholder  string      `json:"placeholder,omitempty"`
}

// SpiderTemplate defines a pre-built spider with metadata, config schema, and code.
type SpiderTemplate struct {
	Id           string            `json:"id"`
	Name         string            `json:"name"`
	Platform     string            `json:"platform"`
	Description  string            `json:"description"`
	Icon         string            `json:"icon"`
	Tags         []string          `json:"tags"`
	ConfigFields []ConfigField     `json:"config_fields"`
	Cmd          string            `json:"cmd"`
	Files        map[string]string `json:"-"` // not sent in listing, only on detail
}

// templateRegistry holds all available templates keyed by id.
var templateRegistry = map[string]*SpiderTemplate{}

func init() {
	register(weiboHotTopicsTemplate())
	register(weiboKeywordSearchTemplate())
	register(weiboUserPostsTemplate())
	register(xhsHotTopicsTemplate())
	register(xhsUserPostsTemplate())
	register(xueqiuHotTopicsTemplate())
	register(xueqiuUserPostsTemplate())
	register(wechatUserArticlesTemplate())
}

func register(t *SpiderTemplate) {
	templateRegistry[t.Id] = t
}

// GetTemplate returns a template by id.
func GetTemplate(id string) *SpiderTemplate {
	return templateRegistry[id]
}

// ListTemplates returns all available templates (without file content).
func ListTemplates() []*SpiderTemplate {
	out := make([]*SpiderTemplate, 0, len(templateRegistry))
	for _, t := range templateRegistry {
		out = append(out, t)
	}
	return out
}

// RenderConfig writes config as config.json bytes.
func RenderConfig(config map[string]interface{}) ([]byte, error) {
	return json.MarshalIndent(config, "", "  ")
}

// ==================== Weibo Hot Topics ====================

func weiboHotTopicsTemplate() *SpiderTemplate {
	minTopics := float64(1)
	maxTopics := float64(100)
	minDelay := float64(0)
	maxDelay := float64(30)

	return &SpiderTemplate{
		Id:          "weibo_hot_topics",
		Name:        "微博热搜",
		Platform:    "weibo",
		Description: "抓取微博实时热搜榜单，包括话题标题、热度值、排名、分类等。支持 Playwright 反爬和 Cookie 复用，适合定时任务监控舆情热点。",
		Icon:        "flame",
		Tags:        []string{"微博", "热搜", "舆情", "Playwright"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "max_topics", Label: "最大话题数", Type: "number", Default: 50, Required: false, Description: "抓取热搜榜前N条话题", Min: &minTopics, Max: &maxTopics},
			{Key: "enable_detail", Label: "抓取链接正文", Type: "boolean", Default: true, Required: false, Description: "是否抓取每个热搜话题链接的正文内容（微博帖子、摘要、互动数据）"},
			{Key: "detail_count", Label: "每话题微博数", Type: "number", Default: 10, Required: false, Description: "每个热搜话题下抓取的微博帖子数量", Min: &minTopics, Max: &maxTopics},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器（关闭可用于调试登录问题）"},
			{Key: "request_delay", Label: "请求间隔(秒)", Type: "number", Default: 2, Required: false, Description: "每次请求之间的等待时间，避免触发反爬", Min: &minDelay, Max: &maxDelay},
			{Key: "cookie", Label: "微博Cookie", Type: "string", Default: "", Required: false, Description: "手动填入微博Cookie（可选，留空则使用Playwright自动获取）", Placeholder: "SUB=xxx; SUBP=xxx;"},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}},
		},
		Files: map[string]string{
			"main.py":          weiboHotTopicsMain,
			"requirements.txt": weiboRequirements,
			"README.md":        weiboHotTopicsReadme,
		},
	}
}

// ==================== Weibo Keyword Search ====================

func weiboKeywordSearchTemplate() *SpiderTemplate {
	minCount := float64(1)
	maxCount := float64(500)
	minDelay := float64(0)
	maxDelay := float64(30)

	return &SpiderTemplate{
		Id:          "weibo_keyword_search",
		Name:        "微博关键词搜索",
		Platform:    "weibo",
		Description: "根据关键词搜索微博内容，抓取微博正文、作者信息、互动数据（点赞/转发/评论数）。支持多关键词批量搜索，Playwright 反爬。",
		Icon:        "search",
		Tags:        []string{"微博", "关键词", "搜索", "Playwright"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "keywords", Label: "搜索关键词", Type: "string", Default: "", Required: true, Description: "搜索关键词，多个关键词用逗号分隔", Placeholder: "人工智能,ChatGPT,大模型"},
			{Key: "max_posts", Label: "每关键词最大条数", Type: "number", Default: 50, Required: false, Description: "每个关键词最多抓取的微博数量", Min: &minCount, Max: &maxCount},
			{Key: "enable_comments", Label: "抓取评论", Type: "boolean", Default: false, Required: false, Description: "是否抓取每条微博的评论"},
			{Key: "max_comments", Label: "每条最大评论数", Type: "number", Default: 20, Required: false, Description: "每条微博最多抓取的评论数量", Min: &minCount, Max: &maxCount},
			{Key: "sort_by", Label: "排序方式", Type: "select", Default: "hot", Required: false, Description: "搜索结果排序", Options: []string{"hot", "time", "default"}},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器"},
			{Key: "request_delay", Label: "请求间隔(秒)", Type: "number", Default: 3, Required: false, Description: "请求间隔时间", Min: &minDelay, Max: &maxDelay},
			{Key: "cookie", Label: "微博Cookie", Type: "string", Default: "", Required: false, Description: "手动填入微博Cookie（可选）", Placeholder: "SUB=xxx; SUBP=xxx;"},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}},
		},
		Files: map[string]string{
			"main.py":          weiboKeywordSearchMain,
			"requirements.txt": weiboRequirements,
			"README.md":        weiboKeywordSearchReadme,
		},
	}
}

// ==================== Spider Code ====================

const weiboRequirements = `playwright>=1.40.0
playwright-stealth>=2.0.0
`

const weiboHotTopicsMain = `#!/usr/bin/env python3
"""
Spider Lab - 微博热搜爬虫
Weibo Hot Topics Crawler

Uses Playwright with stealth to visit weibo.com desktop site.
Fetches hot search list via Ajax API, then optionally scrapes
topic detail pages for related posts.
"""
import asyncio
import csv
import json
import os
import re
import time
from pathlib import Path
from urllib.parse import quote

from playwright.async_api import async_playwright
from playwright_stealth import Stealth

TASK_ID = os.getenv("SPIDER_LAB_TASK_ID", "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR = WORK_DIR / "output" / TASK_ID

TAG_RE = re.compile(r"<[^>]+>")

DESKTOP_UA = ("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
              "AppleWebKit/537.36 (KHTML, like Gecko) "
              "Chrome/120.0.0.0 Safari/537.36")


def strip_html(text):
    return TAG_RE.sub("", text or "").strip()


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def save_results(data, config):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    timestamp = time.strftime("%Y%m%d_%H%M%S")
    fmt = config.get("save_format", "json")
    if fmt == "csv" and data:
        flat_data = []
        for item in data:
            flat = {k: v for k, v in item.items() if not isinstance(v, (dict, list))}
            lc = item.get("link_content", {})
            if isinstance(lc, dict):
                flat["content_summary"] = lc.get("summary", "")
                posts = lc.get("posts", [])
                flat["content_posts_count"] = len(posts)
                flat["content_top_posts"] = " | ".join(
                    p.get("text", "")[:200] for p in posts[:5]
                )
            flat_data.append(flat)
        filepath = OUTPUT_DIR / f"hot_topics_{timestamp}.csv"
        keys = flat_data[0].keys()
        with open(filepath, "w", newline="", encoding="utf-8-sig") as f:
            writer = csv.DictWriter(f, fieldnames=keys)
            writer.writeheader()
            writer.writerows(flat_data)
    else:
        filepath = OUTPUT_DIR / f"hot_topics_{timestamp}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


def parse_cookie_string(cookie_str):
    cookies = []
    if cookie_str:
        for item in cookie_str.split(";"):
            item = item.strip()
            if "=" in item:
                k, v = item.split("=", 1)
                cookies.append({
                    "name": k.strip(), "value": v.strip(),
                    "domain": ".weibo.com", "path": "/",
                })
    return cookies


async def browser_fetch(page, url, max_retries=3):
    for attempt in range(max_retries):
        try:
            result = await page.evaluate(
                """async (url) => {
                    const resp = await fetch(url, {
                        credentials: 'include',
                        headers: {'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest'},
                    });
                    const text = await resp.text();
                    if (!resp.ok) return {__error: true, status: resp.status, text: text.substring(0, 300)};
                    try { return JSON.parse(text); } catch {
                        return {__error: true, status: resp.status, text: text.substring(0, 300)};
                    }
                }""", url)
            if isinstance(result, dict) and result.get("__error"):
                log(f"  API error status={result.get('status')} (attempt {attempt+1})")
                if attempt < max_retries - 1:
                    await asyncio.sleep(2 ** attempt)
                    continue
                return None
            return result
        except Exception as e:
            log(f"  Fetch error: {e} (attempt {attempt+1})")
            if attempt < max_retries - 1:
                await asyncio.sleep(2 ** attempt)
    return None


def extract_post(mblog):
    if not mblog:
        return None
    user = mblog.get("user", {}) or {}
    text = strip_html(mblog.get("text_raw", "")) or strip_html(mblog.get("text", ""))
    if not text:
        return None
    avatar_url = user.get("avatar_hd", "") or user.get("profile_image_url", "")
    images = []
    for pic_id in mblog.get("pic_ids", []):
        pic_info = mblog.get("pic_infos", {}).get(pic_id, {})
        large = pic_info.get("largest", pic_info.get("large", {}))
        img_url = large.get("url", "")
        if img_url:
            images.append(img_url)
    if not images:
        for pic in mblog.get("pics", []):
            large = pic.get("large", {})
            img_url = large.get("url", "") or pic.get("url", "")
            if img_url:
                images.append(img_url)
    return {
        "id": str(mblog.get("id", "")),
        "mid": mblog.get("mid", mblog.get("mblogid", "")),
        "text": text,
        "user_name": user.get("screen_name", ""),
        "user_id": str(user.get("id", "")),
        "avatar_url": avatar_url,
        "verified": user.get("verified", False),
        "images": images,
        "is_long_text": mblog.get("isLongText", False),
        "reposts_count": mblog.get("reposts_count", 0),
        "comments_count": mblog.get("comments_count", 0),
        "attitudes_count": mblog.get("attitudes_count", 0),
        "created_at": mblog.get("created_at", ""),
        "source": strip_html(mblog.get("source", "")),
    }


async def scrape_topic_posts(page, topic_word, detail_count=10, delay=2):
    """Navigate to topic search page and intercept API results."""
    captured = []

    async def on_resp(response):
        url = response.url
        try:
            ct = response.headers.get("content-type", "")
            if "json" in ct and "ajax/" in url:
                body = await response.json()
                captured.append(body)
        except Exception:
            pass

    page.on("response", on_resp)
    try:
        encoded = quote(topic_word)
        url = f"https://s.weibo.com/weibo?q={encoded}&Refer=index"
        log(f"  Scraping topic: {topic_word}")
        await page.goto(url, wait_until="domcontentloaded", timeout=30000)
        await page.wait_for_timeout(5000)

        # Try to extract posts from DOM (s.weibo.com renders server-side)
        posts = []
        dom_posts = await page.evaluate("""() => {
            const items = [];
            document.querySelectorAll('[action-type="feed_list_item"]').forEach(el => {
                const nameEl = el.querySelector('.name');
                const textEl = el.querySelector('[node-type="feed_list_content"]');
                const likeEl = el.querySelector('[action-type="fl_like"] em:last-child');
                const repostEl = el.querySelector('[action-type="fl_forward"] em:last-child');
                const commentEl = el.querySelector('[action-type="fl_comment"] em:last-child');
                items.push({
                    user_name: nameEl?.textContent?.trim() || '',
                    text: textEl?.textContent?.trim() || '',
                    attitudes_count: parseInt(likeEl?.textContent) || 0,
                    reposts_count: parseInt(repostEl?.textContent) || 0,
                    comments_count: parseInt(commentEl?.textContent) || 0,
                });
            });
            return items;
        }""")
        for dp in dom_posts[:detail_count]:
            if dp.get("text"):
                posts.append({
                    "text": dp["text"],
                    "user_name": dp.get("user_name", ""),
                    "attitudes_count": dp.get("attitudes_count", 0),
                    "reposts_count": dp.get("reposts_count", 0),
                    "comments_count": dp.get("comments_count", 0),
                })

        log(f"  Got {len(posts)} posts for: {topic_word}")
        await asyncio.sleep(delay)
        return {"posts": posts}
    except Exception as e:
        log(f"  Warning: Failed to scrape topic {topic_word}: {e}")
        return None
    finally:
        page.remove_listener("response", on_resp)


async def main():
    config = load_config()
    max_topics = int(config.get("max_topics", 50))
    enable_detail = config.get("enable_detail", True)
    detail_count = int(config.get("detail_count", 10))
    headless = config.get("headless", True)
    delay = float(config.get("request_delay", 2))
    manual_cookie = config.get("cookie", "")

    log(f"Spider started — max_topics={max_topics}, enable_detail={enable_detail}")

    pw = await async_playwright().start()
    browser = await pw.chromium.launch(
        headless=headless,
        args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
              "--disable-blink-features=AutomationControlled"],
    )
    context = await browser.new_context(
        user_agent=DESKTOP_UA,
        viewport={"width": 1440, "height": 900},
        locale="zh-CN",
    )

    # Inject cookies if provided
    if manual_cookie:
        cookies = parse_cookie_string(manual_cookie)
        if cookies:
            await context.add_cookies(cookies)
            log(f"Injected {len(cookies)} manual cookies")

    page = await context.new_page()
    await Stealth().apply_stealth_async(page)

    try:
        # Visit weibo.com to establish session
        log("Visiting weibo.com to establish session...")
        try:
            await page.goto("https://weibo.com", wait_until="domcontentloaded", timeout=30000)
            await page.wait_for_timeout(5000)
            cookies = await context.cookies()
            log(f"Session established, {len(cookies)} cookies")
        except Exception as e:
            log(f"Warning: Session setup: {e}")

        # Fetch hot search list via desktop Ajax API
        log("Fetching hot search list...")
        data = await browser_fetch(page, "https://weibo.com/ajax/side/hotSearch")

        if not data or data.get("ok") != 1:
            log("ERROR: Failed to fetch hot search list")
            log(f"Response: {str(data)[:200] if data else 'None'}")
            await browser.close()
            await pw.stop()
            return

        realtime = data.get("data", {}).get("realtime", [])
        log(f"Got {len(realtime)} hot search items")

        hot_topics = []
        all_output = []

        for item in realtime[:max_topics]:
            word = item.get("word", "")
            if not word:
                continue

            # Optionally scrape topic detail (posts)
            content = None
            if enable_detail:
                await asyncio.sleep(delay)
                content = await scrape_topic_posts(
                    page, word, detail_count=detail_count, delay=delay)

            topic = {
                "type": "topic",
                "title": word,
                "rank": len(hot_topics) + 1,
                "hot_value": item.get("raw_hot", item.get("num", "")),
                "category": item.get("category", ""),
                "label_name": item.get("label_name", ""),
                "icon_desc": item.get("icon_desc", ""),
                "summary": "",
                "post_count": len(content.get("posts", [])) if content else 0,
                "task_id": TASK_ID,
                "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
            }
            hot_topics.append(topic)
            all_output.append(topic)
            print(json.dumps(topic, ensure_ascii=False), flush=True)

            # Output each post as separate result item
            if content:
                for post in content.get("posts", []):
                    post["type"] = "post"
                    post["topic_title"] = word
                    post["topic_rank"] = topic["rank"]
                    post["task_id"] = TASK_ID
                    post["crawled_at"] = time.strftime("%Y-%m-%d %H:%M:%S")
                    all_output.append(post)
                    print(json.dumps(post, ensure_ascii=False), flush=True)

            if len(hot_topics) < max_topics:
                await asyncio.sleep(delay)

        log(f"Crawled {len(hot_topics)} hot topics, "
            f"{len(all_output) - len(hot_topics)} posts")
        save_results(all_output, config)

    finally:
        await browser.close()
        await pw.stop()

    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const weiboHotTopicsReadme = `# 微博热搜爬虫 (Weibo Hot Topics)

## 功能
- 抓取微博实时热搜榜（Top 50）
- 自动抓取每个热搜话题链接的正文内容（帖子文本、摘要、互动数据）
- 使用 Playwright 浏览器自动化访问话题页面并拦截 API 响应提取结构化数据
- 支持 Playwright 自动获取 Cookie，绕过反爬
- 支持手动填入 Cookie
- 输出 JSON / CSV 格式

## 配置说明
所有配置通过 Spider Lab UI 设置，保存在 config.json：

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| max_topics | number | 50 | 抓取热搜榜前N条 |
| enable_detail | boolean | true | 是否抓取话题链接正文 |
| detail_count | number | 10 | 每话题微博抓取数 |
| headless | boolean | true | 无头浏览器模式 |
| request_delay | number | 2 | 请求间隔(秒) |
| cookie | string | "" | 手动Cookie(可选) |
| save_format | string | "json" | 输出格式 json/csv |

## 输出字段
每条热搜记录包含：
- rank — 排名
- title — 话题标题
- hot_value — 热度值
- category — 分类
- scheme — 原始链接
- link_content — 链接正文内容
  - summary — 话题摘要
  - posts — 相关微博帖子列表（含文本、作者、转赞评数据）

## 依赖
- playwright (浏览器自动化 + 内容抓取)
- playwright-stealth (反检测)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium

## 输出
数据保存在 output/ 目录，同时每条记录会以 JSON line 输出到 stdout（被 Spider Lab 捕获）。
`

const weiboKeywordSearchMain = `#!/usr/bin/env python3
"""
Spider Lab - 微博关键词搜索爬虫
Weibo Keyword Search Crawler

Uses Playwright with stealth to visit weibo.com desktop site.
Navigates to search result pages and extracts posts from DOM.
"""
import asyncio
import csv
import json
import os
import re
import sys
import time
from pathlib import Path
from urllib.parse import quote

from playwright.async_api import async_playwright
from playwright_stealth import Stealth

TASK_ID = os.getenv("SPIDER_LAB_TASK_ID", "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR = WORK_DIR / "output" / TASK_ID

TAG_RE = re.compile(r"<[^>]+>")

DESKTOP_UA = ("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
              "AppleWebKit/537.36 (KHTML, like Gecko) "
              "Chrome/120.0.0.0 Safari/537.36")


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def strip_html(text):
    return TAG_RE.sub("", text or "").strip()


def save_results(data, config, keyword):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    timestamp = time.strftime("%Y%m%d_%H%M%S")
    safe_kw = re.sub(r'[^\w]', '_', keyword)[:30]
    fmt = config.get("save_format", "json")
    if fmt == "csv" and data:
        filepath = OUTPUT_DIR / f"search_{safe_kw}_{timestamp}.csv"
        keys = data[0].keys()
        with open(filepath, "w", newline="", encoding="utf-8-sig") as f:
            writer = csv.DictWriter(f, fieldnames=keys)
            writer.writeheader()
            writer.writerows(data)
    else:
        filepath = OUTPUT_DIR / f"search_{safe_kw}_{timestamp}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


def parse_cookie_string(cookie_str):
    cookies = []
    if cookie_str:
        for item in cookie_str.split(";"):
            item = item.strip()
            if "=" in item:
                k, v = item.split("=", 1)
                cookies.append({
                    "name": k.strip(), "value": v.strip(),
                    "domain": ".weibo.com", "path": "/",
                })
    return cookies


async def browser_fetch(page, url, max_retries=3):
    for attempt in range(max_retries):
        try:
            result = await page.evaluate(
                """async (url) => {
                    const resp = await fetch(url, {
                        credentials: 'include',
                        headers: {'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest'},
                    });
                    const text = await resp.text();
                    if (!resp.ok) return {__error: true, status: resp.status, text: text.substring(0, 300)};
                    try { return JSON.parse(text); } catch {
                        return {__error: true, status: resp.status, text: text.substring(0, 300)};
                    }
                }""", url)
            if isinstance(result, dict) and result.get("__error"):
                if attempt < max_retries - 1:
                    await asyncio.sleep(2 ** attempt)
                    continue
                return result
            return result
        except Exception as e:
            log(f"  Fetch error: {e} (attempt {attempt+1})")
            if attempt < max_retries - 1:
                await asyncio.sleep(2 ** attempt)
    return None


async def scrape_search_page(page, keyword, page_num=1, sort="hot"):
    """Navigate to s.weibo.com search page and extract posts from DOM."""
    encoded = quote(keyword)
    sort_param = ""
    if sort == "time":
        sort_param = "&timescope=custom::&sort=time"
    elif sort == "hot":
        sort_param = "&xsort=hot"

    url = (f"https://s.weibo.com/weibo?q={encoded}{sort_param}"
           f"&Refer=index&page={page_num}")
    try:
        await page.goto(url, wait_until="domcontentloaded", timeout=30000)
        await page.wait_for_timeout(5000)

        # Check for login redirect
        current_url = page.url
        if "passport.weibo.com" in current_url or "login" in current_url:
            log(f"  Search requires login (redirected to login page)")
            return [], False

        # Extract posts from DOM (s.weibo.com renders server-side)
        posts = await page.evaluate("""() => {
            const items = [];
            document.querySelectorAll('[action-type="feed_list_item"]').forEach(el => {
                const nameEl = el.querySelector('.name');
                const textEl = el.querySelector('[node-type="feed_list_content"]');
                const fullTextEl = el.querySelector('[node-type="feed_list_content_full"]');
                const likeEl = el.querySelector('[action-type="fl_like"] em:last-child');
                const repostEl = el.querySelector('[action-type="fl_forward"] em:last-child');
                const commentEl = el.querySelector('[action-type="fl_comment"] em:last-child');
                const timeEl = el.querySelector('.from a:first-child');
                const sourceEl = el.querySelector('.from a:last-child');
                const imgEls = el.querySelectorAll('[node-type="feed_list_media_prev"] img');

                const images = [];
                imgEls.forEach(img => {
                    let src = img.getAttribute('src') || '';
                    if (src.startsWith('//')) src = 'https:' + src;
                    if (src) images.push(src);
                });

                const displayText = fullTextEl || textEl;
                items.push({
                    user_name: nameEl?.textContent?.trim() || '',
                    text: displayText?.textContent?.trim() || '',
                    attitudes_count: parseInt(likeEl?.textContent) || 0,
                    reposts_count: parseInt(repostEl?.textContent) || 0,
                    comments_count: parseInt(commentEl?.textContent) || 0,
                    created_at: timeEl?.textContent?.trim() || '',
                    source: sourceEl?.textContent?.trim() || '',
                    images: images,
                });
            });
            return items;
        }""")

        # Check if there's a next page
        has_next = await page.evaluate("""() => {
            const next = document.querySelector('.next');
            return next && !next.classList.contains('disabled');
        }""")

        return posts, has_next
    except Exception as e:
        log(f"  Search page error: {e}")
        return [], False


async def fetch_comments_desktop(page, post_id, count=20):
    """Fetch comments via desktop Ajax API."""
    url = (f"https://weibo.com/ajax/statuses/buildComments"
           f"?id={post_id}&is_show_bulletin=2&count={count}")
    data = await browser_fetch(page, url)
    if not data or data.get("ok") != 1:
        return []
    comments = []
    cdata = data.get("data", [])
    if isinstance(cdata, dict):
        cdata = cdata.get("data", [])
    for item in (cdata if isinstance(cdata, list) else []):
        comments.append({
            "id": str(item.get("id", "")),
            "text": strip_html(item.get("text_raw", "")) or strip_html(item.get("text", "")),
            "user_name": item.get("user", {}).get("screen_name", ""),
            "like_count": item.get("like_counts", 0),
            "created_at": item.get("created_at", ""),
        })
        if len(comments) >= count:
            break
    return comments


async def crawl_keyword(page, keyword, config):
    max_posts = int(config.get("max_posts", 50))
    enable_comments = config.get("enable_comments", False)
    max_comments = int(config.get("max_comments", 20))
    sort_by = config.get("sort_by", "hot")
    delay = float(config.get("request_delay", 3))

    log(f"Searching keyword: {keyword} (max={max_posts}, sort={sort_by})")

    all_posts = []
    page_num = 1
    max_pages = (max_posts // 10) + 3

    while len(all_posts) < max_posts and page_num <= max_pages:
        posts, has_next = await scrape_search_page(
            page, keyword, page_num=page_num, sort=sort_by)

        if not posts:
            if page_num == 1:
                log(f"  No results for keyword '{keyword}' "
                    f"(may require login cookie)")
            break

        for dp in posts:
            if len(all_posts) >= max_posts:
                break
            post = {
                "id": "",
                "type": "post",
                "text": dp.get("text", ""),
                "user_name": dp.get("user_name", ""),
                "images": dp.get("images", []),
                "reposts_count": dp.get("reposts_count", 0),
                "comments_count": dp.get("comments_count", 0),
                "attitudes_count": dp.get("attitudes_count", 0),
                "created_at": dp.get("created_at", ""),
                "source": dp.get("source", ""),
                "keyword": keyword,
                "task_id": TASK_ID,
                "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
            }

            if enable_comments and dp.get("comments_count", 0) > 0:
                try:
                    await asyncio.sleep(delay * 0.5)
                    comments = await fetch_comments_desktop(
                        page, post["id"], count=max_comments)
                    post["comments"] = comments
                except Exception:
                    post["comments"] = []

            all_posts.append(post)
            print(json.dumps(post, ensure_ascii=False), flush=True)

        if not has_next:
            break

        page_num += 1
        await asyncio.sleep(delay)

    log(f"  Keyword '{keyword}': collected {len(all_posts)} posts")
    return all_posts


async def main():
    config = load_config()
    keywords_str = config.get("keywords", "")
    headless = config.get("headless", True)
    manual_cookie = config.get("cookie", "")

    if not keywords_str:
        log("ERROR: No keywords configured. Set 'keywords' in spider config.")
        sys.exit(1)

    keywords = [kw.strip() for kw in keywords_str.split(",") if kw.strip()]
    log(f"Spider started — keywords={keywords}")

    pw = await async_playwright().start()
    browser = await pw.chromium.launch(
        headless=headless,
        args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
              "--disable-blink-features=AutomationControlled"],
    )
    context = await browser.new_context(
        user_agent=DESKTOP_UA,
        viewport={"width": 1440, "height": 900},
        locale="zh-CN",
    )

    # Inject cookies if provided
    if manual_cookie:
        cookies = parse_cookie_string(manual_cookie)
        if cookies:
            await context.add_cookies(cookies)
            log(f"Injected {len(cookies)} manual cookies")

    page = await context.new_page()
    await Stealth().apply_stealth_async(page)

    try:
        # Visit weibo.com to establish session
        log("Visiting weibo.com to establish session...")
        try:
            await page.goto("https://weibo.com", wait_until="domcontentloaded", timeout=30000)
            await page.wait_for_timeout(5000)
            cookies = await context.cookies()
            log(f"Session established, {len(cookies)} cookies")
        except Exception as e:
            log(f"Warning: Session setup: {e}")

        all_keyword_posts = []
        for keyword in keywords:
            posts = await crawl_keyword(page, keyword, config)
            all_keyword_posts.extend(posts)
            save_results(posts, config, keyword)

    finally:
        await browser.close()
        await pw.stop()

    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const weiboKeywordSearchReadme = `# 微博关键词搜索爬虫 (Weibo Keyword Search)

## 功能
- 根据关键词搜索微博内容
- 抓取微博正文、作者、互动数据
- 可选抓取评论
- 支持热度/时间排序
- Playwright 自动获取 Cookie

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| keywords | string | "" | 搜索关键词，逗号分隔（必填） |
| max_posts | number | 50 | 每关键词最大微博数 |
| enable_comments | boolean | false | 是否抓取评论 |
| max_comments | number | 20 | 每条最大评论数 |
| sort_by | select | "hot" | 排序: hot/time/default |
| headless | boolean | true | 无头浏览器模式 |
| request_delay | number | 3 | 请求间隔(秒) |
| cookie | string | "" | 手动Cookie(可选) |
| save_format | string | "json" | 输出格式 |

## 依赖
- playwright (浏览器自动化)
- playwright-stealth (反检测)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium

## 输出
数据保存在 output/ 目录，每条记录也以 JSON line 输出到 stdout。
`

// ==================== Weibo User Posts ====================

func weiboUserPostsTemplate() *SpiderTemplate {
	minCount := float64(1)
	maxCount := float64(1000)
	minDelay := float64(0)
	maxDelay := float64(60)

	return &SpiderTemplate{
		Id:          "weibo_user_posts",
		Name:        "微博用户主页",
		Platform:    "weibo",
		Description: "抓取指定微博用户的全部/近期微博，包括正文、图片、互动数据（点赞/转发/评论数）、评论内容。支持图片本地下载（防盗链）、长文本自动展开，适合账号监控和内容归档。",
		Icon:        "user",
		Tags:        []string{"微博", "用户", "主页", "监控"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "user_id", Label: "用户UID", Type: "string", Default: "", Required: true, Description: "微博用户UID（数字），可从用户主页URL获取，如 weibo.com/u/1234567890 中的 1234567890", Placeholder: "1234567890"},
			{Key: "max_posts", Label: "最大微博数", Type: "number", Default: 50, Required: false, Description: "最多抓取的微博条数", Min: &minCount, Max: &maxCount},
			{Key: "download_images", Label: "下载图片", Type: "boolean", Default: true, Required: false, Description: "是否将图片下载到本地（推荐开启，微博有防盗链策略，外链图片可能无法显示）"},
			{Key: "enable_comments", Label: "抓取评论", Type: "boolean", Default: false, Required: false, Description: "是否抓取每条微博的热门评论"},
			{Key: "max_comments", Label: "每条最大评论数", Type: "number", Default: 10, Required: false, Description: "每条微博最多抓取的评论数量", Min: &minCount, Max: &maxCount},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器"},
			{Key: "delay_min", Label: "最小间隔(秒)", Type: "number", Default: 1, Required: false, Description: "请求之间的最小随机等待时间", Min: &minDelay, Max: &maxDelay},
			{Key: "delay_max", Label: "最大间隔(秒)", Type: "number", Default: 5, Required: false, Description: "请求之间的最大随机等待时间", Min: &minDelay, Max: &maxDelay},
			{Key: "cookie", Label: "微博Cookie", Type: "string", Default: "", Required: false, Description: "手动填入微博Cookie（推荐提供，用户微博接口需要登录态）", Placeholder: "SUB=xxx; SUBP=xxx;"},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}},
		},
		Files: map[string]string{
			"main.py":          weiboUserPostsMain,
			"requirements.txt": weiboRequirements,
			"README.md":        weiboUserPostsReadme,
		},
	}
}

const weiboUserPostsMain = `#!/usr/bin/env python3
"""
Spider Lab - 微博用户主页爬虫
Weibo User Posts Crawler

Uses Playwright with stealth to visit weibo.com desktop site.
Fetches user profile via Ajax API, then navigates to user page
and fetches posts via desktop Ajax API.
"""
import asyncio
import base64
import csv
import hashlib
import json
import os
import random
import re
import sys
import time
import urllib.request
from pathlib import Path
from urllib.parse import urlparse

from playwright.async_api import async_playwright
from playwright_stealth import Stealth

TASK_ID = os.getenv("SPIDER_LAB_TASK_ID", "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR = WORK_DIR / "output" / TASK_ID
IMAGES_DIR = WORK_DIR / "images"

TAG_RE = re.compile(r"<[^>]+>")

DESKTOP_UA = ("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
              "AppleWebKit/537.36 (KHTML, like Gecko) "
              "Chrome/120.0.0.0 Safari/537.36")


def strip_html(text):
    return TAG_RE.sub("", text or "").strip()


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def rand_delay(delay_min, delay_max):
    if delay_min >= delay_max:
        return delay_min
    return random.uniform(delay_min, delay_max)


def save_results(data, config, screen_name):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    timestamp = time.strftime("%Y%m%d_%H%M%S")
    safe_name = re.sub(r'[^\w]', '_', screen_name)[:30]
    fmt = config.get("save_format", "json")
    if fmt == "csv" and data:
        filepath = OUTPUT_DIR / f"user_{safe_name}_{timestamp}.csv"
        flat_data = []
        for item in data:
            flat = {k: v for k, v in item.items()
                    if not isinstance(v, (dict, list))}
            flat["images"] = " | ".join(
                img.get("url", "") for img in item.get("images", []))
            flat["local_images"] = " | ".join(
                img.get("local_path", "") for img in item.get("images", [])
                if img.get("local_path"))
            flat["comments_preview"] = " | ".join(
                c.get("text", "")[:60] for c in item.get("comments", [])[:3])
            flat_data.append(flat)
        keys = list(flat_data[0].keys()) if flat_data else []
        with open(filepath, "w", newline="", encoding="utf-8-sig") as f:
            writer = csv.DictWriter(f, fieldnames=keys)
            writer.writeheader()
            writer.writerows(flat_data)
    else:
        filepath = OUTPUT_DIR / f"user_{safe_name}_{timestamp}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


def parse_cookie_string(cookie_str):
    cookies = []
    if cookie_str:
        for item in cookie_str.split(";"):
            item = item.strip()
            if "=" in item:
                k, v = item.split("=", 1)
                cookies.append({
                    "name": k.strip(), "value": v.strip(),
                    "domain": ".weibo.com", "path": "/",
                })
    return cookies


async def browser_fetch(page, url, max_retries=3):
    for attempt in range(max_retries):
        try:
            result = await page.evaluate(
                """async (url) => {
                    const resp = await fetch(url, {
                        credentials: 'include',
                        headers: {'Accept': 'application/json', 'X-Requested-With': 'XMLHttpRequest'},
                    });
                    const text = await resp.text();
                    if (!resp.ok) return {__error: true, status: resp.status, text: text.substring(0, 300)};
                    try { return JSON.parse(text); } catch {
                        return {__error: true, status: resp.status, text: text.substring(0, 300)};
                    }
                }""", url)
            if isinstance(result, dict) and result.get("__error"):
                if attempt < max_retries - 1:
                    await asyncio.sleep(2 ** attempt)
                    continue
                return result
            return result
        except Exception as e:
            log(f"  Fetch error: {e} (attempt {attempt+1})")
            if attempt < max_retries - 1:
                await asyncio.sleep(2 ** attempt)
    return None


async def download_image(page, url, post_id, index):
    """Download image locally with Weibo referer headers."""
    try:
        parsed = urlparse(url)
        ext = Path(parsed.path).suffix or ".jpg"
        url_hash = hashlib.md5(url.encode()).hexdigest()[:8]
        filename = f"{post_id}_{index}_{url_hash}{ext}"
        local_path = IMAGES_DIR / filename
        if local_path.exists():
            return str(local_path)
        # Try urllib with proper headers
        req = urllib.request.Request(url, headers={
            "User-Agent": DESKTOP_UA,
            "Referer": "https://weibo.com/",
        })
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = resp.read()
            if len(data) > 100:
                local_path.write_bytes(data)
                return str(local_path)
    except Exception:
        pass
    # Fallback: download via browser
    try:
        b64 = await page.evaluate(
            """async (url) => {
                try {
                    const r = await fetch(url, {credentials:'include'});
                    if (!r.ok) return null;
                    const b = await r.blob();
                    return new Promise(res => {
                        const rd = new FileReader();
                        rd.onloadend = () => res(rd.result.split(',')[1]);
                        rd.readAsDataURL(b);
                    });
                } catch { return null; }
            }""", url)
        if b64:
            local_path = IMAGES_DIR / filename
            local_path.write_bytes(base64.b64decode(b64))
            return str(local_path)
    except Exception:
        pass
    return None


async def main():
    config = load_config()
    uid = str(config.get("user_id", "")).strip()
    max_posts = int(config.get("max_posts", 50))
    download_images = config.get("download_images", True)
    enable_comments = config.get("enable_comments", False)
    max_comments = int(config.get("max_comments", 10))
    headless = config.get("headless", True)
    delay_min = float(config.get("delay_min", 1))
    delay_max = float(config.get("delay_max", 5))
    manual_cookie = config.get("cookie", "")

    if delay_min > delay_max:
        delay_min, delay_max = delay_max, delay_min

    if not uid:
        log("ERROR: user_id is required. Set it in spider config.")
        sys.exit(1)

    log(f"Spider started — uid={uid}, max_posts={max_posts}, "
        f"download_images={download_images}, delay={delay_min}~{delay_max}s")

    if download_images:
        IMAGES_DIR.mkdir(parents=True, exist_ok=True)

    pw = await async_playwright().start()
    browser = await pw.chromium.launch(
        headless=headless,
        args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
              "--disable-blink-features=AutomationControlled"],
    )
    context = await browser.new_context(
        user_agent=DESKTOP_UA,
        viewport={"width": 1440, "height": 900},
        locale="zh-CN",
    )

    # Inject cookies if provided
    if manual_cookie:
        cookies = parse_cookie_string(manual_cookie)
        if cookies:
            await context.add_cookies(cookies)
            log(f"Injected {len(cookies)} manual cookies")

    page = await context.new_page()
    await Stealth().apply_stealth_async(page)

    try:
        # Visit weibo.com to establish session
        log("Visiting weibo.com to establish session...")
        try:
            await page.goto("https://weibo.com", wait_until="domcontentloaded",
                            timeout=30000)
            await page.wait_for_timeout(5000)
            cookies = await context.cookies()
            log(f"Session established, {len(cookies)} cookies")
        except Exception as e:
            log(f"Warning: Session setup: {e}")

        # Navigate to user profile to capture page-level session
        log(f"Visiting user profile: weibo.com/u/{uid}")
        try:
            await page.goto(f"https://weibo.com/u/{uid}",
                            wait_until="domcontentloaded", timeout=30000)
            await page.wait_for_timeout(5000)
        except Exception as e:
            log(f"Warning: Profile page load: {e}")

        # Fetch user profile via Ajax API
        log(f"Fetching user profile for UID {uid}...")
        screen_name = uid
        avatar_url = ""
        verified = False
        verified_type = -1
        try:
            profile_data = await browser_fetch(
                page, f"https://weibo.com/ajax/profile/info?uid={uid}")
            if profile_data and profile_data.get("ok") == 1:
                user_info = profile_data.get("data", {}).get("user", {})
                screen_name = user_info.get("screen_name", uid)
                avatar_url = (user_info.get("avatar_hd", "")
                              or user_info.get("profile_image_url", ""))
                verified = user_info.get("verified", False)
                verified_type = user_info.get("verified_type", -1)
                description = user_info.get("description", "")
                followers = user_info.get("followers_count", 0)
                following = user_info.get("follow_count", 0)
                statuses_count = user_info.get("statuses_count", 0)

                log(f"User: {screen_name} | Followers: {followers} | "
                    f"Posts: {statuses_count}")

                # Download avatar
                local_avatar = None
                if download_images and avatar_url:
                    local_avatar = await download_image(
                        page, avatar_url, f"avatar_{uid}", 0)

                profile = {
                    "type": "profile",
                    "user_id": uid,
                    "user_name": screen_name,
                    "avatar_url": avatar_url,
                    "avatar_local": local_avatar,
                    "verified": verified,
                    "verified_type": verified_type,
                    "description": description,
                    "followers_count": followers,
                    "follow_count": following,
                    "statuses_count": statuses_count,
                    "task_id": TASK_ID,
                    "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                }
                print(json.dumps(profile, ensure_ascii=False), flush=True)
            else:
                log(f"Warning: Profile API returned: {str(profile_data)[:200]}")
        except Exception as e:
            log(f"Warning: Failed to fetch user info: {e}")

        # Fetch posts via desktop Ajax API
        all_posts = []
        page_num = 1
        since_id = ""
        consecutive_empty = 0
        total_images_downloaded = 0
        seen_ids = set()

        while len(all_posts) < max_posts:
            url = (f"https://weibo.com/ajax/statuses/mymblog"
                   f"?uid={uid}&page={page_num}&feature=0")
            if since_id:
                url += f"&since_id={since_id}"

            data = await browser_fetch(page, url)
            if not data or (isinstance(data, dict) and data.get("__error")):
                err_text = ""
                if isinstance(data, dict):
                    err_text = data.get("text", "")
                log(f"Page {page_num} fetch failed: {err_text}")
                if "403" in str(err_text) or "Forbidden" in str(err_text):
                    log("ERROR: Posts API requires login. "
                        "Please provide a valid cookie in spider config.")
                    break
                consecutive_empty += 1
                if consecutive_empty >= 3:
                    break
                page_num += 1
                await asyncio.sleep(rand_delay(delay_min, delay_max))
                continue

            if data.get("ok") != 1:
                msg = data.get("message", "")
                log(f"Page {page_num} returned ok={data.get('ok')}: {msg}")
                if not msg:
                    log("Posts API may require login. "
                        "Please provide a valid cookie in spider config.")
                consecutive_empty += 1
                if consecutive_empty >= 2:
                    break
                page_num += 1
                await asyncio.sleep(rand_delay(delay_min, delay_max))
                continue

            post_data = data.get("data", {})
            posts_list = post_data.get("list", [])
            next_since_id = post_data.get("since_id", "")
            if next_since_id:
                since_id = str(next_since_id)

            if not posts_list:
                log(f"No more posts at page {page_num}")
                consecutive_empty += 1
                if consecutive_empty >= 2:
                    break
                page_num += 1
                await asyncio.sleep(rand_delay(delay_min, delay_max))
                continue

            consecutive_empty = 0
            found_any = False

            for mblog in posts_list:
                post_id = str(mblog.get("id", ""))
                if not post_id or post_id in seen_ids:
                    continue
                seen_ids.add(post_id)

                text = (strip_html(mblog.get("text_raw", ""))
                        or strip_html(mblog.get("text", "")))
                if not text:
                    continue

                # Extract and download images
                images = []
                pic_ids = mblog.get("pic_ids", [])
                pic_infos = mblog.get("pic_infos", {})
                for i, pic_id in enumerate(pic_ids):
                    info = pic_infos.get(pic_id, {})
                    large = info.get("largest", info.get("large", {}))
                    img_url = large.get("url", "")
                    if not img_url:
                        continue
                    img_entry = {"url": img_url, "local_path": None}
                    if download_images:
                        local_path = await download_image(
                            page, img_url, post_id, i)
                        if local_path:
                            img_entry["local_path"] = local_path
                            total_images_downloaded += 1
                        await asyncio.sleep(
                            rand_delay(delay_min * 0.2, delay_max * 0.3))
                    images.append(img_entry)

                # Fallback image extraction
                if not images:
                    for i, pic in enumerate(mblog.get("pics", [])):
                        large = pic.get("large", {})
                        img_url = large.get("url", "") or pic.get("url", "")
                        if not img_url:
                            continue
                        img_entry = {"url": img_url, "local_path": None}
                        if download_images:
                            local_path = await download_image(
                                page, img_url, post_id, i)
                            if local_path:
                                img_entry["local_path"] = local_path
                                total_images_downloaded += 1
                        images.append(img_entry)

                user = mblog.get("user", {}) or {}
                post = {
                    "type": "post",
                    "id": post_id,
                    "mid": mblog.get("mid", mblog.get("mblogid", "")),
                    "text": text,
                    "user_name": user.get("screen_name", screen_name),
                    "user_id": uid,
                    "avatar_url": avatar_url,
                    "verified": verified,
                    "verified_type": verified_type,
                    "images": images,
                    "is_long_text": mblog.get("isLongText", False),
                    "reposts_count": mblog.get("reposts_count", 0),
                    "comments_count": mblog.get("comments_count", 0),
                    "attitudes_count": mblog.get("attitudes_count", 0),
                    "created_at": mblog.get("created_at", ""),
                    "source": strip_html(mblog.get("source", "")),
                    "task_id": TASK_ID,
                    "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                }

                # Fetch full text for long posts
                if mblog.get("isLongText", False):
                    try:
                        await asyncio.sleep(
                            rand_delay(delay_min * 0.3, delay_max * 0.5))
                        mblogid = mblog.get("mblogid", post_id)
                        lt_data = await browser_fetch(
                            page,
                            f"https://weibo.com/ajax/statuses/longtext"
                            f"?id={mblogid}")
                        if (lt_data and lt_data.get("ok") == 1):
                            long_text = lt_data.get("data", {}).get(
                                "longTextContent", "")
                            if long_text:
                                post["full_text"] = strip_html(long_text)
                    except Exception as e:
                        log(f"  Failed to fetch long text for {post_id}: {e}")

                # Fetch comments
                if enable_comments and mblog.get("comments_count", 0) > 0:
                    try:
                        await asyncio.sleep(
                            rand_delay(delay_min * 0.5, delay_max * 0.5))
                        cmt_data = await browser_fetch(
                            page,
                            f"https://weibo.com/ajax/statuses/buildComments"
                            f"?id={post_id}&is_show_bulletin=2"
                            f"&count={max_comments}")
                        comments = []
                        if cmt_data and cmt_data.get("ok") == 1:
                            cdata = cmt_data.get("data", [])
                            if isinstance(cdata, dict):
                                cdata = cdata.get("data", [])
                            for c in (cdata if isinstance(cdata, list) else []):
                                comments.append({
                                    "id": str(c.get("id", "")),
                                    "text": (strip_html(c.get("text_raw", ""))
                                             or strip_html(c.get("text", ""))),
                                    "user_name": c.get("user", {}).get(
                                        "screen_name", ""),
                                    "like_count": c.get("like_counts", 0),
                                    "created_at": c.get("created_at", ""),
                                })
                                if len(comments) >= max_comments:
                                    break
                        post["comments"] = comments
                        log(f"  Got {len(comments)} comments for {post_id}")
                    except Exception as e:
                        log(f"  Failed to fetch comments for {post_id}: {e}")
                        post["comments"] = []

                all_posts.append(post)
                print(json.dumps(post, ensure_ascii=False), flush=True)
                found_any = True

                if len(all_posts) >= max_posts:
                    break

            if not found_any:
                consecutive_empty += 1
                if consecutive_empty >= 2:
                    break

            page_num += 1
            log(f"Progress: {len(all_posts)}/{max_posts} posts "
                f"(page {page_num - 1}), "
                f"{total_images_downloaded} images downloaded")
            await asyncio.sleep(rand_delay(delay_min, delay_max))

        log(f"Collected {len(all_posts)} posts from @{screen_name}, "
            f"{total_images_downloaded} images downloaded")
        save_results(all_posts, config, screen_name)

    finally:
        await browser.close()
        await pw.stop()

    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const weiboUserPostsReadme = `# 微博用户主页爬虫 (Weibo User Posts)

## 功能
- 抓取指定微博用户的全部/近期微博
- 自动获取用户资料（头像、粉丝数、简介等）
- 抓取微博正文、图片、互动数据（点赞/转发/评论数）
- 图片自动下载到本地，绕过微博防盗链策略
- 长文本自动展开获取完整内容
- 可选抓取每条微博的热门评论
- 使用 Playwright + stealth 反检测
- 通过桌面版 weibo.com Ajax API 获取数据
- 随机请求间隔，模拟人工行为避免反爬
- 输出 JSON / CSV 格式

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| user_id | string | "" | 微博用户UID（必填） |
| max_posts | number | 50 | 最大微博抓取条数 |
| download_images | boolean | true | 下载图片到本地（推荐开启） |
| enable_comments | boolean | false | 是否抓取评论 |
| max_comments | number | 10 | 每条最大评论数 |
| headless | boolean | true | 无头浏览器模式 |
| delay_min | number | 1 | 最小随机请求间隔(秒) |
| delay_max | number | 5 | 最大随机请求间隔(秒) |
| cookie | string | "" | 手动Cookie（用户微博API需要登录，推荐提供） |
| save_format | string | "json" | 输出格式 |

## 重要说明
微博用户微博接口 (ajax/statuses/mymblog) 需要登录才能访问。
建议在配置中提供有效的 Cookie 字符串以获取完整数据。
用户资料接口 (ajax/profile/info) 无需登录即可使用。

## 图片下载
开启 download_images 后，所有微博图片会下载到 images/ 目录。
每张图片以 {post_id}_{index}_{hash}.jpg 命名，避免重复下载。
图片下载使用正确的 Referer 头，绕过微博 CDN 防盗链。

## 如何获取用户 UID
1. 打开微博用户主页
2. URL 中的数字即为 UID，如 weibo.com/u/1234567890 中的 1234567890

## 输出
- 用户资料（type=profile）：头像、粉丝数、关注数、简介
- 用户微博（type=post）：正文、图片（含本地路径）、互动数据、评论

## 依赖
- playwright (浏览器自动化)
- playwright-stealth (反检测)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium
`
