package spider

// ==================== Xueqiu Hot Topics ====================

func xueqiuHotTopicsTemplate() *SpiderTemplate {
	minItems := float64(1)
	maxItems := float64(100)
	minDelay := float64(0)
	maxDelay := float64(30)

	return &SpiderTemplate{
		Id:          "xueqiu_hot_topics",
		Name:        "雪球热门",
		Platform:    "xueqiu",
		Description: "抓取雪球平台热门内容，包括7x24财经直播、热门帖子讨论。使用 Playwright 绕过阿里云 WAF 防护，通过 API 获取结构化数据，支持下载帖子图片到本地，适合金融舆情监控。",
		Icon:        "flame",
		Tags:        []string{"雪球", "热门", "财经", "Playwright"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "enable_live_feed", Label: "抓取7x24直播", Type: "boolean", Default: true, Required: false, Description: "是否抓取7x24小时财经直播"},
			{Key: "enable_hot_posts", Label: "抓取热门帖子", Type: "boolean", Default: true, Required: false, Description: "是否抓取热门用户帖子"},
			{Key: "max_items", Label: "每类最大条数", Type: "number", Default: 30, Required: false, Description: "每个类别最多抓取的条数", Min: &minItems, Max: &maxItems},
			{Key: "download_images", Label: "下载图片", Type: "boolean", Default: true, Required: false, Description: "是否下载帖子中的图片和头像到本地"},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器"},
			{Key: "request_delay", Label: "请求间隔(秒)", Type: "number", Default: 2, Required: false, Description: "API 请求间隔时间", Min: &minDelay, Max: &maxDelay},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}},
		},
		Files: map[string]string{
			"main.py":          xueqiuHotTopicsMain,
			"requirements.txt": xueqiuRequirements,
			"README.md":        xueqiuHotTopicsReadme,
		},
	}
}

// ==================== Xueqiu User Posts ====================

func xueqiuUserPostsTemplate() *SpiderTemplate {
	minCount := float64(1)
	maxCount := float64(500)
	minDelay := float64(0)
	maxDelay := float64(30)

	return &SpiderTemplate{
		Id:          "xueqiu_user_posts",
		Name:        "雪球用户帖子",
		Platform:    "xueqiu",
		Description: "抓取指定雪球用户的全部/近期帖子，包括帖子内容、互动数据（回复/转发/点赞数）、转帖来源等。使用 Playwright 绕过 WAF，支持下载图片到本地，适合大V观点追踪和投资分析。",
		Icon:        "user",
		Tags:        []string{"雪球", "用户", "帖子", "投资", "Playwright"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "user_id", Label: "用户ID", Type: "string", Default: "", Required: true, Description: "雪球用户ID（数字），可从用户主页URL获取，如 xueqiu.com/u/1234567890 中的 1234567890", Placeholder: "1247347556"},
			{Key: "max_posts", Label: "最大帖子数", Type: "number", Default: 50, Required: false, Description: "最多抓取的帖子条数", Min: &minCount, Max: &maxCount},
			{Key: "download_images", Label: "下载图片", Type: "boolean", Default: true, Required: false, Description: "是否下载帖子中的图片和头像到本地"},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器"},
			{Key: "request_delay", Label: "请求间隔(秒)", Type: "number", Default: 2, Required: false, Description: "请求间隔时间", Min: &minDelay, Max: &maxDelay},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}},
		},
		Files: map[string]string{
			"main.py":          xueqiuUserPostsMain,
			"requirements.txt": xueqiuRequirements,
			"README.md":        xueqiuUserPostsReadme,
		},
	}
}

// ==================== Xueqiu Spider Code ====================

const xueqiuRequirements = `playwright>=1.40.0
playwright-stealth>=2.0.0
`

const xueqiuHotTopicsMain = `#!/usr/bin/env python3
"""
Spider Lab - 雪球热门爬虫 (Xueqiu Hot Topics)

Uses Playwright to bypass Aliyun WAF. Each data source uses a page
on the correct subdomain to avoid CORS and per-route WAF challenges:
  - xueqiu.com for 7x24 live feed
  - xueqiu.com for hot posts (with DOM extraction fallback)
"""
import asyncio
import base64
import csv
import json
import os
import re
import time
import urllib.request
from pathlib import Path

from playwright.async_api import async_playwright
from playwright_stealth import Stealth

TASK_ID = os.getenv("SPIDER_LAB_TASK_ID", "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR = WORK_DIR / "output" / TASK_ID
IMAGE_DIR = WORK_DIR / "images"

TAG_RE = re.compile(r"<[^>]+>")
IMG_RE = re.compile(r'<img[^>]+src=["\x27]([^"\x27]+)["\x27]', re.IGNORECASE)


def strip_html(text):
    return TAG_RE.sub("", text or "").strip()


def extract_image_urls(html_text):
    if not html_text:
        return []
    return [u for u in IMG_RE.findall(html_text) if u.startswith("http")]


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def get_ext(url):
    for ext in (".png", ".gif", ".webp", ".jpeg"):
        if ext in url.lower():
            return ext
    return ".jpg"


async def download_image(page, url, save_path):
    Path(save_path).parent.mkdir(parents=True, exist_ok=True)
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 Chrome/120.0.0.0",
            "Referer": "https://xueqiu.com/",
        })
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = resp.read()
            if len(data) > 0:
                with open(save_path, "wb") as f:
                    f.write(data)
                return True
    except Exception:
        pass
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
            with open(save_path, "wb") as f:
                f.write(base64.b64decode(b64))
            return True
    except Exception:
        pass
    return False


async def download_post_images(page, image_urls, post_id, cache):
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    results = []  # list of (local_path, api_url)
    for i, url in enumerate(image_urls[:10]):
        if url in cache:
            results.append(cache[url])
            continue
        fname = f"post_{post_id}_{i}{get_ext(url)}"
        save_path = str(IMAGE_DIR / fname)
        api_url = f"/api/spiders/{SPIDER_ID}/images/{fname}"
        if await download_image(page, url, save_path):
            entry = (save_path, api_url)
            results.append(entry)
            cache[url] = entry
    return results


async def download_avatar(page, user_info, cache):
    pd = user_info.get("photo_domain", "")
    pu = user_info.get("profile_image_url", "")
    if not pd or not pu:
        return ""
    url = pd + pu
    if url in cache:
        return cache[url]
    uid = user_info.get("id", "unknown")
    fname = f"avatar_{uid}{get_ext(url)}"
    save_path = str(IMAGE_DIR / fname)
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    if await download_image(page, url, save_path):
        cache[url] = save_path
        return save_path
    return ""


def save_results(data, config, label):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    ts = time.strftime("%Y%m%d_%H%M%S")
    fmt = config.get("save_format", "json")
    if fmt == "csv" and data:
        filepath = OUTPUT_DIR / f"xueqiu_{label}_{ts}.csv"
        flat = []
        for item in data:
            row = {k: (json.dumps(v, ensure_ascii=False) if isinstance(v, (dict, list)) else v)
                   for k, v in item.items()}
            flat.append(row)
        keys = list(flat[0].keys()) if flat else []
        with open(filepath, "w", newline="", encoding="utf-8-sig") as f:
            w = csv.DictWriter(f, fieldnames=keys)
            w.writeheader()
            w.writerows(flat)
    else:
        filepath = OUTPUT_DIR / f"xueqiu_{label}_{ts}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


async def browser_fetch(page, url, max_retries=3):
    """Fetch JSON API from within the browser context."""
    for attempt in range(max_retries):
        try:
            result = await page.evaluate(
                """async (url) => {
                    const resp = await fetch(url, {
                        credentials: 'include',
                        headers: {
                            'Accept': 'application/json, text/plain, */*',
                            'X-Requested-With': 'XMLHttpRequest',
                        },
                    });
                    const text = await resp.text();
                    const isWaf = text.includes('aliyun_waf') || text.includes('_waf_bd8ce2ce37') || text.trimStart().startsWith('<');
                    if (!resp.ok || isWaf) {
                        return {__error: true, status: resp.status, waf: isWaf, text: text.substring(0, 200)};
                    }
                    try { return JSON.parse(text); } catch {
                        return {__error: true, status: resp.status, text: text.substring(0, 200)};
                    }
                }""", url)
            if isinstance(result, dict) and result.get("__error"):
                is_waf = result.get("waf", False)
                log(f"  API error status={result.get('status')}"
                    f"{' [WAF]' if is_waf else ''} (attempt {attempt+1})")
                if is_waf and attempt < max_retries - 1:
                    log("  Re-resolving WAF via homepage...")
                    try:
                        await page.goto("https://xueqiu.com/", wait_until="networkidle", timeout=30000)
                        await page.wait_for_timeout(3000)
                    except Exception:
                        pass
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


async def main():
    config = load_config()
    enable_live_feed = config.get("enable_live_feed", True)
    enable_hot_posts = config.get("enable_hot_posts", True)
    max_items = int(config.get("max_items", 30))
    headless = config.get("headless", True)
    delay = float(config.get("request_delay", 2))
    cookie_str = config.get("cookie", "")
    do_images = config.get("download_images", True)

    log(f"Spider started — live={enable_live_feed}, "
        f"posts={enable_hot_posts}, max={max_items}, images={do_images}")

    all_results = []
    img_cache = {}

    async with async_playwright() as p:
        browser = await p.chromium.launch(
            headless=headless,
            args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
                  "--disable-blink-features=AutomationControlled"],
        )
        ctx = await browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                       "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
            viewport={"width": 1440, "height": 900},
            locale="zh-CN",
        )

        # Inject cookies from pool
        if cookie_str:
            cookies = []
            for item in cookie_str.split(";"):
                item = item.strip()
                if "=" in item:
                    k, v = item.split("=", 1)
                    cookies.append({
                        "name": k.strip(), "value": v.strip(),
                        "domain": ".xueqiu.com", "path": "/",
                    })
            if cookies:
                await ctx.add_cookies(cookies)
                log(f"Injected {len(cookies)} pool cookies")

        # Open main xueqiu.com page — used for live feed and as base
        page = await ctx.new_page()
        await Stealth().apply_stealth_async(page)

        # Check if pool cookies already provide xq_a_token (skip WAF if so)
        existing = await ctx.cookies()
        has_token = any(c["name"] == "xq_a_token" for c in existing)
        if has_token:
            log(f"Pool cookies include xq_a_token, skipping WAF resolution ({len(existing)} cookies)")
            # Still need to navigate to xueqiu.com as base page for API calls
            try:
                await page.goto("https://xueqiu.com/", wait_until="domcontentloaded", timeout=30000)
                await page.wait_for_timeout(2000)
            except Exception:
                pass
        else:
            log("Visiting xueqiu.com to pass WAF...")
            try:
                await page.goto("https://xueqiu.com/", wait_until="networkidle", timeout=60000)
                await page.wait_for_timeout(3000)
                for _ in range(5):
                    cookies = await ctx.cookies()
                    if any(c["name"] == "xq_a_token" for c in cookies):
                        break
                    await page.wait_for_timeout(2000)
                log(f"WAF passed, {len(await ctx.cookies())} cookies")
            except Exception as e:
                log(f"ERROR: WAF failed: {e}")
                await browser.close()
                return

        # === 7x24 Live Feed ===
        if enable_live_feed:
            log("Fetching 7x24 live feed...")
            url = (f"https://xueqiu.com/statuses/livenews/list.json"
                   f"?since_id=-1&max_id=-1&count={max_items}")
            data = await browser_fetch(page, url)
            if data:
                items = data.get("items", []) or data.get("data", {}).get("items", [])
                for i, news in enumerate(items[:max_items]):
                    text_html = news.get("text", "")
                    image_urls = extract_image_urls(text_html)
                    local_images = []
                    api_images = []
                    if do_images and image_urls:
                        dl_results = await download_post_images(
                            page, image_urls, f"news_{i}", img_cache)
                        local_images = [r[0] for r in dl_results]
                        api_images = [r[1] for r in dl_results]
                    item = {
                        "type": "live_news", "rank": i + 1,
                        "user_name": "7x24",
                        "text": strip_html(text_html),
                        "created_at": news.get("created_at", ""),
                        "source": news.get("source", ""),
                        "target": news.get("target", ""),
                        "image_urls": image_urls,
                        "images": api_images if api_images else image_urls,
                        "local_images": local_images,
                        "task_id": TASK_ID,
                        "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                    }
                    all_results.append(item)
                    print(json.dumps(item, ensure_ascii=False), flush=True)
                log(f"  Got {len(items)} live news items")
            else:
                log("  Failed to fetch live feed")
            await asyncio.sleep(delay)

        # === Hot Posts (via direct API fetch) ===
        if enable_hot_posts:
            log("Fetching hot posts via API...")
            try:
                collected_posts = []
                next_max_id = -1
                while len(collected_posts) < max_items:
                    url = (f"https://xueqiu.com/statuses/hot/listV2.json"
                           f"?since_id=-1&max_id={next_max_id}&size=15")
                    data = await browser_fetch(page, url)
                    if not data or (isinstance(data, dict) and data.get("__error")):
                        log(f"  Hot posts API error: {data}")
                        break
                    items = data.get("items", [])
                    if not items:
                        break
                    next_max_id = data.get("next_max_id", -1)
                    for pd in items:
                        if len(collected_posts) >= max_items:
                            break
                        collected_posts.append(pd)
                    if next_max_id == -1:
                        break
                    await asyncio.sleep(delay)

                log(f"  Fetched {len(collected_posts)} hot posts via API")
                for i, pd in enumerate(collected_posts[:max_items]):
                    orig = pd.get("original_status", {}) or pd
                    user = orig.get("user", {}) or {}
                    text_html = orig.get("text", "")
                    image_urls = extract_image_urls(text_html)
                    local_images = []
                    api_images = []
                    local_avatar = ""
                    avatar_url = ""
                    if user.get("photo_domain") and user.get("profile_image_url"):
                        avatar_url = user["photo_domain"] + user["profile_image_url"]
                    if do_images:
                        if image_urls:
                            dl_results = await download_post_images(
                                page, image_urls, f"hot_{orig.get('id', i)}", img_cache)
                            local_images = [r[0] for r in dl_results]
                            api_images = [r[1] for r in dl_results]
                        if avatar_url:
                            local_avatar = await download_avatar(page, user, img_cache)
                    item = {
                        "type": "hot_post", "rank": i + 1,
                        "id": str(orig.get("id", "")),
                        "title": orig.get("title", ""),
                        "text": strip_html(text_html)[:500],
                        "description": orig.get("description", ""),
                        "user_name": user.get("screen_name", ""),
                        "user_id": str(user.get("id", "")),
                        "reply_count": orig.get("reply_count", 0),
                        "retweet_count": orig.get("retweet_count", 0),
                        "like_count": orig.get("like_count", 0) or orig.get("fav_count", 0),
                        "created_at": orig.get("created_at", ""),
                        "image_urls": image_urls,
                        "images": api_images if api_images else image_urls,
                        "local_images": local_images,
                        "avatar_url": avatar_url,
                        "local_avatar": local_avatar,
                        "task_id": TASK_ID,
                        "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                    }
                    all_results.append(item)
                    print(json.dumps(item, ensure_ascii=False), flush=True)
            except Exception as e:
                log(f"  Hot posts error: {e}")

        await browser.close()

    img_count = len(img_cache)

    log(f"Total: {len(all_results)} items, {img_count} images downloaded")
    save_results(all_results, config, "hot")

    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const xueqiuHotTopicsReadme = `# 雪球热门爬虫 (Xueqiu Hot Topics)

## 功能
- 抓取 7x24 小时财经直播
- 抓取热门用户帖子讨论
- 自动下载帖子中的图片和用户头像到本地
- 使用 Playwright 绕过阿里云 WAF 反爬
- 通过 API 获取结构化数据
- 输出 JSON / CSV 格式

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| enable_live_feed | boolean | true | 抓取7x24直播 |
| enable_hot_posts | boolean | true | 抓取热门帖子 |
| max_items | number | 30 | 每类别最大条数 |
| download_images | boolean | true | 下载图片到本地 |
| headless | boolean | true | 无头浏览器模式 |
| request_delay | number | 2 | 请求间隔(秒) |
| save_format | string | "json" | 输出格式 json/csv |

## 图片下载
开启 download_images 后，图片保存在 output/images/ 目录：
- 用户头像: avatar_{user_id}.jpg
- 帖子图片: post_{post_id}_{index}.jpg
结果数据中 local_images / local_avatar 字段为本地路径。

## WAF 说明
雪球使用阿里云 WAF 防护，普通 HTTP 请求会被拦截。
本爬虫使用 Playwright 浏览器全程驻留，通过 page.evaluate(fetch())
在浏览器上下文内发起 API 请求，确保 WAF 会话始终有效。

## 依赖
- playwright (浏览器自动化 + WAF bypass + API 请求)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium

## 输出
数据保存在 output/ 目录，图片保存在 output/images/ 目录。
每条记录也以 JSON line 输出到 stdout。
`

const xueqiuUserPostsMain = `#!/usr/bin/env python3
"""
Spider Lab - 雪球用户帖子爬虫 (Xueqiu User Posts)

Uses Playwright to bypass Aliyun WAF. Navigates to the user's profile
page and intercepts API responses for structured data, with fetch()
fallback for pagination.
"""
import asyncio
import base64
import csv
import json
import os
import re
import time
import urllib.request
from pathlib import Path

from playwright.async_api import async_playwright
from playwright_stealth import Stealth

TASK_ID = os.getenv("SPIDER_LAB_TASK_ID", "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR = WORK_DIR / "output" / TASK_ID
IMAGE_DIR = WORK_DIR / "images"

TAG_RE = re.compile(r"<[^>]+>")
IMG_RE = re.compile(r'<img[^>]+src=["\x27]([^"\x27]+)["\x27]', re.IGNORECASE)


def strip_html(text):
    return TAG_RE.sub("", text or "").strip()


def extract_image_urls(html_text):
    if not html_text:
        return []
    return [u for u in IMG_RE.findall(html_text) if u.startswith("http")]


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def get_ext(url):
    for ext in (".png", ".gif", ".webp", ".jpeg"):
        if ext in url.lower():
            return ext
    return ".jpg"


async def download_image(page, url, save_path):
    Path(save_path).parent.mkdir(parents=True, exist_ok=True)
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 Chrome/120.0.0.0",
            "Referer": "https://xueqiu.com/",
        })
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = resp.read()
            if len(data) > 0:
                with open(save_path, "wb") as f:
                    f.write(data)
                return True
    except Exception:
        pass
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
            with open(save_path, "wb") as f:
                f.write(base64.b64decode(b64))
            return True
    except Exception:
        pass
    return False


async def download_post_images(page, image_urls, post_id, cache):
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    results = []  # list of (local_path, api_url)
    for i, url in enumerate(image_urls[:10]):
        if url in cache:
            results.append(cache[url])
            continue
        fname = f"post_{post_id}_{i}{get_ext(url)}"
        save_path = str(IMAGE_DIR / fname)
        api_url = f"/api/spiders/{SPIDER_ID}/images/{fname}"
        if await download_image(page, url, save_path):
            entry = (save_path, api_url)
            results.append(entry)
            cache[url] = entry
    return results


async def download_avatar_url(page, avatar_url, uid, cache):
    if not avatar_url:
        return ""
    if avatar_url in cache:
        return cache[avatar_url]
    fname = f"avatar_{uid}{get_ext(avatar_url)}"
    save_path = str(IMAGE_DIR / fname)
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    if await download_image(page, avatar_url, save_path):
        cache[avatar_url] = save_path
        return save_path
    return ""


def save_results(data, config, user_name):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    ts = time.strftime("%Y%m%d_%H%M%S")
    safe_name = re.sub(r"[^\w]", "_", user_name)[:30]
    fmt = config.get("save_format", "json")
    if fmt == "csv" and data:
        filepath = OUTPUT_DIR / f"xueqiu_user_{safe_name}_{ts}.csv"
        flat = []
        for item in data:
            row = {k: (json.dumps(v, ensure_ascii=False) if isinstance(v, (dict, list)) else v)
                   for k, v in item.items()}
            flat.append(row)
        keys = list(flat[0].keys()) if flat else []
        with open(filepath, "w", newline="", encoding="utf-8-sig") as f:
            w = csv.DictWriter(f, fieldnames=keys)
            w.writeheader()
            w.writerows(flat)
    else:
        filepath = OUTPUT_DIR / f"xueqiu_user_{safe_name}_{ts}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


async def resolve_waf(page, ctx):
    """Navigate to xueqiu.com homepage and wait for xq_a_token cookie."""
    log("Resolving WAF challenge...")
    try:
        await page.goto("https://xueqiu.com/", wait_until="networkidle", timeout=60000)
        await page.wait_for_timeout(3000)
        for _ in range(10):
            cookies = await ctx.cookies()
            if any(c["name"] == "xq_a_token" for c in cookies):
                log(f"WAF resolved, {len(cookies)} cookies")
                return True
            await page.wait_for_timeout(2000)
        log("WARNING: xq_a_token not found after WAF resolution attempt")
        return False
    except Exception as e:
        log(f"WAF resolution error: {e}")
        return False


async def browser_fetch(page, url, ctx=None, max_retries=3):
    """Fetch JSON API from within the browser context, with WAF recovery."""
    for attempt in range(max_retries):
        try:
            result = await page.evaluate(
                """async (url) => {
                    const resp = await fetch(url, {
                        credentials: 'include',
                        headers: {
                            'Accept': 'application/json, text/plain, */*',
                            'X-Requested-With': 'XMLHttpRequest',
                        },
                    });
                    const text = await resp.text();
                    const isWaf = text.includes('aliyun_waf') || text.includes('_waf_bd8ce2ce37') || text.trimStart().startsWith('<');
                    if (!resp.ok || isWaf) {
                        return {__error: true, status: resp.status, waf: isWaf, text: text.substring(0, 200)};
                    }
                    try { return JSON.parse(text); } catch {
                        return {__error: true, status: resp.status, text: text.substring(0, 200)};
                    }
                }""", url)
            if isinstance(result, dict) and result.get("__error"):
                is_waf = result.get("waf", False)
                log(f"  API error status={result.get('status')}"
                    f"{' [WAF]' if is_waf else ''} (attempt {attempt+1})")
                if is_waf and ctx and attempt < max_retries - 1:
                    await resolve_waf(page, ctx)
                    continue
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


async def fetch_full_text(page, post_id, ctx=None):
    """Fetch full post HTML from statuses/show.json when text is truncated."""
    url = f"https://xueqiu.com/statuses/show.json?id={post_id}"
    data = await browser_fetch(page, url, ctx=ctx)
    if data and not data.get("error_code"):
        status = data.get("status") or data
        text = status.get("text", "")
        if text:
            return text
    return None


async def main():
    config = load_config()
    user_id = str(config.get("user_id", "")).strip()
    max_posts = int(config.get("max_posts", 50))
    headless = config.get("headless", True)
    delay = float(config.get("request_delay", 2))
    do_images = config.get("download_images", True)
    cookie_str = config.get("cookie", "")

    if not user_id:
        log("ERROR: user_id is required. Set it in spider config.")
        return

    log(f"Spider started — user_id={user_id}, max_posts={max_posts}, "
        f"images={do_images}")

    all_posts = []
    user_name = user_id
    avatar_url = ""
    local_avatar = ""
    img_cache = {}
    captured_statuses = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(
            headless=headless,
            args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
                  "--disable-blink-features=AutomationControlled"],
        )
        ctx = await browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                       "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
            viewport={"width": 1440, "height": 900},
            locale="zh-CN",
        )

        # Inject cookies if provided
        if cookie_str:
            cookies = []
            for item in cookie_str.split(";"):
                item = item.strip()
                if "=" in item:
                    k, v = item.split("=", 1)
                    cookies.append({
                        "name": k.strip(), "value": v.strip(),
                        "domain": ".xueqiu.com", "path": "/",
                    })
            if cookies:
                await ctx.add_cookies(cookies)
                log(f"Injected {len(cookies)} pool cookies")

        page = await ctx.new_page()
        await Stealth().apply_stealth_async(page)

        # Step 1: Check if pool cookies already provide xq_a_token (skip WAF if so)
        existing = await ctx.cookies()
        has_token = any(c["name"] == "xq_a_token" for c in existing)
        if has_token:
            log(f"Pool cookies include xq_a_token, skipping WAF resolution ({len(existing)} cookies)")
        else:
            log("No xq_a_token in cookies, resolving WAF challenge...")
            if not await resolve_waf(page, ctx):
                log("WARNING: Could not establish WAF session, continuing anyway...")

        # Step 2: Navigate to user profile page for user info extraction
        profile_url = f"https://xueqiu.com/u/{user_id}"
        log(f"Visiting user profile: {profile_url}")
        try:
            await page.goto(profile_url, wait_until="domcontentloaded", timeout=60000)
            await page.wait_for_timeout(5000)
            # Verify WAF session is still valid
            cookies = await ctx.cookies()
            has_token = any(c["name"] == "xq_a_token" for c in cookies)
            if not has_token:
                log("WAF token lost after profile navigation, re-resolving...")
                await resolve_waf(page, ctx)
                await page.goto(profile_url, wait_until="domcontentloaded", timeout=60000)
                await page.wait_for_timeout(5000)
            cookies = await ctx.cookies()
            log(f"Profile loaded, {len(cookies)} cookies")
        except Exception as e:
            log(f"ERROR: Failed to load profile: {e}")
            await browser.close()
            return

        # Extract user info from page
        try:
            user_info = await page.evaluate("""() => {
                const nameEl = document.querySelector('.profile-name-cn, .name--KBL3l, h1.name');
                const descEl = document.querySelector('.profile-description, .description--KdGYD');
                const avatarEl = document.querySelector('.profile-avatar img, .avatar--jyRtI img');
                const fansEl = document.querySelector('[href*="fans"] .count, .fan-num');
                return {
                    name: nameEl?.textContent?.trim() || '',
                    desc: descEl?.textContent?.trim() || '',
                    avatar: avatarEl?.src || '',
                    fans_text: fansEl?.textContent?.trim() || '',
                };
            }""")
            if user_info.get("name"):
                user_name = user_info["name"]
                avatar_url = user_info.get("avatar", "")
                if do_images and avatar_url:
                    local_avatar = await download_avatar_url(
                        page, avatar_url, user_id, img_cache)
                profile = {
                    "type": "profile",
                    "user_id": user_id,
                    "user_name": user_name,
                    "avatar_url": avatar_url,
                    "local_avatar": local_avatar,
                    "description": user_info.get("desc", ""),
                    "fans_text": user_info.get("fans_text", ""),
                    "task_id": TASK_ID,
                    "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                }
                print(json.dumps(profile, ensure_ascii=False), flush=True)
                log(f"User: {user_name}")
        except Exception:
            pass

        # Also try the user/show.json API (may work on this page's context)
        if user_name == user_id:
            show_url = f"https://xueqiu.com/user/show.json?id={user_id}"
            info = await browser_fetch(page, show_url, ctx=ctx)
            if info and not info.get("error_code"):
                user_name = info.get("screen_name", user_id)

        # Fetch posts via two complementary APIs:
        # 1) v4/user_timeline page 1: most recent 20 posts (all types incl. reposts)
        # 2) original/timeline pages 1-N: original posts only, ~5 pages without login
        log("Fetching posts via API...")
        seen_ids = set()

        # Phase 1: v4 user_timeline page 1 (includes reposts)
        v4_url = (f"https://xueqiu.com/v4/statuses/user_timeline.json"
                  f"?user_id={user_id}&page=1&page_size=20")
        v4_data = await browser_fetch(page, v4_url, ctx=ctx)
        if v4_data and not v4_data.get("error_code") and not v4_data.get("__error"):
            v4_statuses = v4_data.get("statuses", [])
            for s in v4_statuses:
                sid = str(s.get("id", ""))
                if sid and sid not in seen_ids:
                    seen_ids.add(sid)
                    captured_statuses.append(s)
            log(f"  v4 page 1: {len(v4_statuses)} posts "
                f"(total={v4_data.get('total', '?')})")
        else:
            err = ""
            if v4_data:
                err = v4_data.get("error_description", "") or v4_data.get("text", "")
            log(f"  v4 page 1 failed: {err or 'unknown'}")

        # Phase 1b: v4 pagination (works with login cookie)
        if len(captured_statuses) < max_posts and v4_data and not v4_data.get("error_code"):
            v4_page = 2
            while len(captured_statuses) < max_posts:
                url = (f"https://xueqiu.com/v4/statuses/user_timeline.json"
                       f"?user_id={user_id}&page={v4_page}&page_size=20")
                data = await browser_fetch(page, url, ctx=ctx)
                if not data or data.get("__error"):
                    break
                if data.get("error_code"):
                    log(f"  v4 page {v4_page}: {data.get('error_description', 'login required')} "
                        f"(需要登录Cookie才能翻页)")
                    break
                statuses = data.get("statuses", [])
                if not statuses:
                    break
                added = 0
                for s in statuses:
                    sid = str(s.get("id", ""))
                    if sid and sid not in seen_ids:
                        seen_ids.add(sid)
                        captured_statuses.append(s)
                        added += 1
                log(f"  v4 page {v4_page}: +{added} posts, {len(captured_statuses)} total")
                if added == 0:
                    break
                v4_page += 1
                await asyncio.sleep(delay)

        # Phase 2: original/timeline for more pages (original content only)
        if len(captured_statuses) < max_posts:
            page_num = 1
            consecutive_empty = 0
            while len(captured_statuses) < max_posts:
                url = (f"https://xueqiu.com/statuses/original/timeline.json"
                       f"?user_id={user_id}&page={page_num}&count=20")
                data = await browser_fetch(page, url, ctx=ctx)
                if not data or data.get("__error"):
                    consecutive_empty += 1
                    if consecutive_empty >= 2:
                        break
                    page_num += 1
                    await asyncio.sleep(delay)
                    continue
                if data.get("error_code"):
                    log(f"  original/timeline page {page_num}: "
                        f"{data.get('error_description', 'error')}")
                    break
                posts_list = data.get("list", [])
                if not posts_list:
                    if page_num == 1:
                        log("  original/timeline: no original posts for this user")
                    break
                consecutive_empty = 0
                added = 0
                for item in posts_list:
                    sid = str(item.get("id", ""))
                    if not sid or sid in seen_ids:
                        continue
                    seen_ids.add(sid)
                    # Normalize original/timeline format to match v4
                    desc = item.get("description", "")
                    pic_str = item.get("pic", "")
                    image_list = [u.strip() for u in pic_str.split(",")
                                  if u.strip()] if pic_str else []
                    text_html = desc
                    if image_list:
                        text_html += "".join(
                            f'<img src="{u}">' for u in image_list)
                    normalized = {
                        "id": item.get("id"),
                        "title": item.get("title", ""),
                        "text": text_html,
                        "description": desc,
                        "view_count": item.get("view_count", 0),
                        "created_at": item.get("created_at", 0),
                        "reply_count": item.get("reply_count", 0),
                        "retweet_count": item.get("retweet_count", 0),
                        "like_count": item.get("like_count", 0),
                        "fav_count": item.get("fav_count", 0),
                        "source": item.get("source", ""),
                    }
                    captured_statuses.append(normalized)
                    added += 1
                log(f"  original/timeline page {page_num}: +{added} posts, "
                    f"{len(captured_statuses)} total")
                if added == 0:
                    break
                page_num += 1
                await asyncio.sleep(delay)

        log(f"Total collected: {len(captured_statuses)} posts"
            + (" (提示：Cookie池中无可用Cookie，仅获取最近20条)"
               if not cookie_str and len(captured_statuses) <= 20
               else ""))

        # Sort by created_at descending (newest first) for chronological timeline
        def sort_key(s):
            ts = s.get("created_at", 0)
            if isinstance(ts, (int, float)):
                return ts
            return 0
        captured_statuses.sort(key=sort_key, reverse=True)

        # Process captured posts
        seen_ids = set()
        for status in captured_statuses:
            if len(all_posts) >= max_posts:
                break
            sid = str(status.get("id", ""))
            if sid in seen_ids:
                continue
            seen_ids.add(sid)

            text_html = status.get("text", "")

            # Fetch full content if text is truncated (ends with ... or …)
            plain_check = strip_html(text_html)
            if plain_check.endswith("...") or plain_check.endswith("\u2026"):
                full_html = await fetch_full_text(page, sid, ctx=ctx)
                if full_html:
                    text_html = full_html
                    log(f"  Post {sid}: fetched full content ({len(full_html)} chars)")
                await asyncio.sleep(0.5)

            image_urls = extract_image_urls(text_html)
            local_images = []
            api_images = []
            if do_images and image_urls:
                dl_results = await download_post_images(
                    page, image_urls, sid, img_cache)
                local_images = [r[0] for r in dl_results]
                api_images = [r[1] for r in dl_results]

            # Replace remote image URLs with API-accessible paths in content
            content_html = text_html
            if api_images:
                for remote_url, api_url in zip(image_urls, api_images):
                    content_html = content_html.replace(remote_url, api_url)

            post = {
                "type": "post",
                "id": sid,
                "title": status.get("title", ""),
                "text": strip_html(text_html),
                "text_html": content_html,
                "description": status.get("description", ""),
                "user_name": user_name,
                "user_id": user_id,
                "avatar_url": avatar_url,
                "local_avatar": local_avatar,
                "reply_count": status.get("reply_count", 0),
                "retweet_count": status.get("retweet_count", 0),
                "like_count": status.get("like_count", 0) or status.get("fav_count", 0),
                "view_count": status.get("view_count", 0),
                "created_at": status.get("created_at", ""),
                "source": status.get("source", ""),
                "image_urls": image_urls,
                "images": api_images if api_images else image_urls,
                "local_images": local_images,
                "task_id": TASK_ID,
                "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
            }

            retweet = status.get("retweeted_status")
            if retweet:
                rt_user = retweet.get("user", {}) or {}
                post["retweet_from"] = rt_user.get("screen_name", "")
                rt_html = retweet.get("text", "")
                post["retweet_text"] = strip_html(rt_html)[:500]

            all_posts.append(post)
            print(json.dumps(post, ensure_ascii=False), flush=True)

        await browser.close()

    img_count = len(img_cache)
    log(f"Collected {len(all_posts)} posts from {user_name}, {img_count} images downloaded")
    save_results(all_posts, config, user_name)

    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const xueqiuUserPostsReadme = `# 雪球用户帖子爬虫 (Xueqiu User Posts)

## 功能
- 抓取指定雪球用户的全部/近期帖子
- 获取用户资料（头像、粉丝数、简介等）
- 自动下载帖子图片和用户头像到本地
- 提取转帖来源和内容
- 使用 Playwright + stealth 绕过阿里云 WAF 反爬
- 通过 v4 user_timeline API 按时间线分页获取
- 自动从 Cookie 池获取登录 Cookie 以获取全部历史帖子
- 输出 JSON / CSV 格式

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| user_id | string | (必填) | 雪球用户ID |
| max_posts | number | 50 | 最多抓取帖子数 |
| download_images | boolean | true | 下载图片到本地 |
| headless | boolean | true | 无头浏览器模式 |
| request_delay | number | 2 | 请求间隔(秒) |
| save_format | string | "json" | 输出格式 json/csv |

## 重要说明
雪球用户帖子 API 在未登录时仅返回最近 20 条帖子。
本爬虫会自动从 Cookie 池获取登录 Cookie 以支持翻页获取全部历史帖子。
请确保 Cookie 池中有可用的雪球 Cookie（通过系统设置页面的扫码登录添加）。

## 图片下载
开启 download_images 后，图片保存在 images/ 目录：
- 用户头像: avatar_{user_id}.jpg
- 帖子图片: post_{post_id}_{index}.jpg
- 转帖图片: post_rt_{post_id}_{index}.jpg

## 获取用户ID
访问用户主页 https://xueqiu.com/u/1234567890，URL 中的数字即为用户ID。

## WAF 说明
雪球使用阿里云 WAF 防护，普通 HTTP 请求会被拦截。
本爬虫使用 Playwright 浏览器全程驻留，通过 page.evaluate(fetch())
在浏览器上下文内发起 API 请求，确保 WAF 会话始终有效。

## 依赖
- playwright (浏览器自动化 + WAF bypass + API 请求)
- playwright-stealth (反检测)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium

## 输出
数据保存在 output/ 目录，图片保存在 images/ 目录。
每条记录也以 JSON line 输出到 stdout。
`
