package spider

// ==================== XHS (Xiaohongshu) Hot Topics ====================

func xhsHotTopicsTemplate() *SpiderTemplate {
	minNotes := float64(1)
	maxNotes := float64(200)
	minDelay := float64(0)
	maxDelay := float64(30)

	return &SpiderTemplate{
		Id:          "xhs_hot_topics",
		Name:        "小红书热门笔记",
		Platform:    "xhs",
		Description: "抓取小红书发现页热门笔记，包括笔记标题、作者、点赞数、封面图、笔记类型（图文/视频）等。使用 Playwright 浏览器拦截 API 响应获取结构化数据，支持滚动加载更多内容。",
		Icon:        "flame",
		Tags:        []string{"小红书", "热门", "发现", "笔记", "Playwright"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "max_notes", Label: "最大笔记数", Type: "number", Default: 30, Required: false, Description: "抓取发现页前N条笔记", Min: &minNotes, Max: &maxNotes},
			{Key: "download_images", Label: "下载图片", Type: "boolean", Default: true, Required: false, Description: "是否下载笔记封面图和用户头像到本地"},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器（关闭可用于调试）"},
			{Key: "request_delay", Label: "滚动间隔(秒)", Type: "number", Default: 2, Required: false, Description: "每次滚动加载的等待间隔", Min: &minDelay, Max: &maxDelay},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}},
		},
		Files: map[string]string{
			"main.py":          xhsHotTopicsMain,
			"requirements.txt": xhsRequirements,
			"README.md":        xhsHotTopicsReadme,
		},
	}
}

// ==================== XHS User Posts ====================

func xhsUserPostsTemplate() *SpiderTemplate {
	minNotes := float64(1)
	maxNotes := float64(500)
	minDelay := float64(0)
	maxDelay := float64(30)

	return &SpiderTemplate{
		Id:          "xhs_user_posts",
		Name:        "小红书用户笔记",
		Platform:    "xhs",
		Description: "抓取指定小红书用户的全部/近期笔记，包括笔记标题、封面、点赞数等。使用 Playwright 浏览器自动化，通过拦截 API 响应获取结构化数据，适合博主内容归档和竞品分析。",
		Icon:        "user",
		Tags:        []string{"小红书", "用户", "笔记", "Playwright"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "user_id", Label: "用户ID", Type: "string", Default: "", Required: true, Description: "小红书用户ID（可从用户主页URL获取，如 xiaohongshu.com/user/profile/xxx 中的 xxx）", Placeholder: "5fa7492e00000000010049ef"},
			{Key: "max_notes", Label: "最大笔记数", Type: "number", Default: 30, Required: false, Description: "最多抓取的笔记条数", Min: &minNotes, Max: &maxNotes},
			{Key: "download_images", Label: "下载图片", Type: "boolean", Default: true, Required: false, Description: "是否下载笔记封面图和用户头像到本地"},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器（关闭可用于调试）"},
			{Key: "request_delay", Label: "请求间隔(秒)", Type: "number", Default: 2, Required: false, Description: "滚动加载间隔时间", Min: &minDelay, Max: &maxDelay},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}},
		},
		Files: map[string]string{
			"main.py":          xhsUserPostsMain,
			"requirements.txt": xhsRequirements,
			"README.md":        xhsUserPostsReadme,
		},
	}
}

// ==================== XHS Spider Code ====================

const xhsRequirements = `playwright>=1.40.0
`

const xhsHotTopicsMain = `#!/usr/bin/env python3
"""
Spider Lab - 小红书热门笔记爬虫
XHS (Xiaohongshu) Hot Topics / Explore Crawler

Uses Playwright to load the XHS explore page and intercept the homefeed
API responses to extract trending notes. Supports downloading cover
images and user avatars to local storage.
"""
import asyncio
import csv
import json
import os
import re
import time
import urllib.request
from pathlib import Path

from playwright.async_api import async_playwright

TASK_ID = os.getenv("SPIDER_LAB_TASK_ID", "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR = WORK_DIR / "output" / TASK_ID
IMAGE_DIR = WORK_DIR / "images"


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def parse_count(count_str):
    """Parse XHS count string like '8.1万' to integer."""
    if not count_str:
        return 0
    if isinstance(count_str, (int, float)):
        return int(count_str)
    s = str(count_str).strip()
    if "万" in s:
        try:
            return int(float(s.replace("万", "")) * 10000)
        except ValueError:
            return 0
    try:
        return int(s)
    except ValueError:
        return 0


def get_ext(url):
    for ext in (".png", ".gif", ".webp", ".jpeg"):
        if ext in url.lower():
            return ext
    return ".jpg"


def download_image(url, save_path):
    """Download image via direct HTTP request."""
    Path(save_path).parent.mkdir(parents=True, exist_ok=True)
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                          "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
            "Referer": "https://www.xiaohongshu.com/",
        })
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = resp.read()
            if len(data) > 0:
                with open(save_path, "wb") as f:
                    f.write(data)
                return True
    except Exception:
        pass
    return False


def download_cover(url, note_id, cache):
    """Download note cover image, return local path."""
    if not url or not url.startswith("http"):
        return ""
    if url in cache:
        return cache[url]
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    fname = f"cover_{note_id}{get_ext(url)}"
    save_path = str(IMAGE_DIR / fname)
    if download_image(url, save_path):
        cache[url] = save_path
        log(f"    Cover saved: {fname}")
        return save_path
    return ""


def download_avatar(url, user_id, cache):
    """Download user avatar, return local path."""
    if not url or not url.startswith("http"):
        return ""
    if url in cache:
        return cache[url]
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    fname = f"avatar_{user_id}{get_ext(url)}"
    save_path = str(IMAGE_DIR / fname)
    if download_image(url, save_path):
        cache[url] = save_path
        return save_path
    return ""


def save_results(data, config):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    ts = time.strftime("%Y%m%d_%H%M%S")
    fmt = config.get("save_format", "json")
    if fmt == "csv" and data:
        filepath = OUTPUT_DIR / f"xhs_explore_{ts}.csv"
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
        filepath = OUTPUT_DIR / f"xhs_explore_{ts}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


def extract_note(feed_item, do_images, img_cache):
    """Extract note data from a feed item (API response or SSR data)."""
    note_card = feed_item.get("note_card") or feed_item.get("noteCard") or {}
    if not note_card and feed_item.get("model_type") == "note":
        note_card = feed_item
    if not note_card:
        return None

    user = note_card.get("user", {})
    interact = note_card.get("interact_info", {}) or note_card.get("interactInfo", {})
    cover = note_card.get("cover", {})

    nid = (feed_item.get("id", "") or feed_item.get("note_id", "")
           or note_card.get("note_id", ""))
    title = (note_card.get("display_title", "") or note_card.get("displayTitle", "")
             or note_card.get("title", ""))
    note_type = note_card.get("type", "")

    if not nid:
        return None

    liked_count = interact.get("liked_count") or interact.get("likedCount") or 0
    user_id = user.get("user_id", "") or user.get("userId", "")
    user_avatar = user.get("avatar", "")

    cover_url = ""
    if isinstance(cover.get("info_list"), list) and cover.get("info_list"):
        cover_url = cover["info_list"][0].get("url", "")
    if not cover_url:
        cover_url = (cover.get("url_default", "") or cover.get("urlDefault", "")
                     or cover.get("url", "") or cover.get("urlPre", ""))

    local_cover = ""
    local_avatar = ""
    if do_images:
        if cover_url:
            local_cover = download_cover(cover_url, nid, img_cache)
        if user_avatar:
            local_avatar = download_avatar(user_avatar, user_id or "unknown", img_cache)

    return {
        "type": "note",
        "note_id": nid,
        "title": title,
        "note_type": note_type,
        "user_id": user_id,
        "user_name": (user.get("nick_name", "") or user.get("nickname", "")
                      or user.get("nickName", "")),
        "user_avatar": user_avatar,
        "local_avatar": local_avatar,
        "liked_count": parse_count(liked_count),
        "liked_count_raw": str(liked_count),
        "cover_url": cover_url,
        "local_cover": local_cover,
        "cover_width": cover.get("width", 0),
        "cover_height": cover.get("height", 0),
        "note_url": f"https://www.xiaohongshu.com/explore/{nid}",
        "task_id": TASK_ID,
        "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
    }


async def main():
    config = load_config()
    max_notes = int(config.get("max_notes", 30))
    headless = config.get("headless", True)
    delay = float(config.get("request_delay", 2))
    do_images = config.get("download_images", True)

    log(f"Spider started — max_notes={max_notes}, images={do_images}")

    seen_ids = set()
    all_notes = []
    img_cache = {}

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=headless)
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                       "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
            viewport={"width": 1440, "height": 900},
            locale="zh-CN",
        )

        async def on_response(response):
            url = response.url
            try:
                if "/api/sns/web/v1/homefeed" in url:
                    body = await response.json()
                    if body.get("success") or body.get("code") == 0:
                        items = body.get("data", {}).get("items", [])
                        for item in items:
                            nid = item.get("id", "") or item.get("note_id", "")
                            if nid and nid not in seen_ids:
                                seen_ids.add(nid)
                                note = extract_note(item, do_images, img_cache)
                                if note:
                                    note["rank"] = len(all_notes) + 1
                                    all_notes.append(note)
                                    print(json.dumps(note, ensure_ascii=False), flush=True)
            except Exception:
                pass

        page = await context.new_page()
        page.on("response", on_response)

        log("Visiting XHS explore page...")
        try:
            await page.goto("https://www.xiaohongshu.com/explore",
                            wait_until="networkidle", timeout=30000)
            await page.wait_for_timeout(5000)
            log(f"  Initial load: {len(all_notes)} notes")

            if len(all_notes) == 0:
                try:
                    state = await page.evaluate("() => window.__INITIAL_STATE__")
                    if state:
                        feeds = state.get("feed", {}).get("feeds", [])
                        for feed in feeds:
                            nid = feed.get("id", "")
                            if nid and nid not in seen_ids:
                                seen_ids.add(nid)
                                note = extract_note(feed, do_images, img_cache)
                                if note:
                                    note["rank"] = len(all_notes) + 1
                                    all_notes.append(note)
                                    print(json.dumps(note, ensure_ascii=False), flush=True)
                        if all_notes:
                            log(f"  Got {len(all_notes)} notes from __INITIAL_STATE__")
                except Exception:
                    pass

            max_scroll = max(max_notes // 20 + 5, 8)
            for scroll_i in range(max_scroll):
                if len(all_notes) >= max_notes:
                    break
                await page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
                await page.wait_for_timeout(int(delay * 1000))
                log(f"  Scroll {scroll_i+1}: {len(all_notes)} notes")

        except Exception as e:
            log(f"Error: {e}")
        finally:
            await browser.close()

    all_notes = all_notes[:max_notes]
    img_count = len(img_cache)
    log(f"Collected {len(all_notes)} notes, {img_count} images downloaded")
    save_results(all_notes, config)
    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const xhsHotTopicsReadme = `# 小红书热门笔记爬虫 (XHS Hot Topics)

## 功能
- 抓取小红书发现页（Explore）热门笔记
- 使用 Playwright 浏览器加载页面，拦截 homefeed API 响应
- 支持滚动加载更多笔记
- 自动下载笔记封面图和用户头像到本地
- 提取笔记标题、作者、点赞数、封面图、笔记类型等
- 同时尝试从 __INITIAL_STATE__ 提取数据作为备选方案
- 输出 JSON / CSV 格式

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| max_notes | number | 30 | 抓取发现页前N条笔记 |
| download_images | boolean | true | 下载图片到本地 |
| headless | boolean | true | 无头浏览器模式 |
| request_delay | number | 2 | 滚动加载间隔(秒) |
| save_format | string | "json" | 输出格式 json/csv |

## 图片下载
开启 download_images 后，图片保存在 output/images/ 目录：
- 笔记封面: cover_{note_id}.jpg
- 用户头像: avatar_{user_id}.jpg
结果数据中 local_cover / local_avatar 字段为本地路径。

## 输出字段
- note_id — 笔记ID
- title — 标题
- note_type — 类型（normal=图文, video=视频）
- user_name — 作者昵称
- user_id — 作者ID
- liked_count — 点赞数
- cover_url — 封面图URL
- local_cover — 本地封面路径
- note_url — 笔记链接
- rank — 排名

## 依赖
- playwright (浏览器自动化 + API 拦截)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium

## 输出
数据保存在 output/ 目录，图片保存在 output/images/ 目录。
每条记录也以 JSON line 输出到 stdout。
`

const xhsUserPostsMain = `#!/usr/bin/env python3
"""
Spider Lab - 小红书用户笔记爬虫
XHS (Xiaohongshu) User Posts Crawler

Uses Playwright to browse a user's profile page and intercept API
responses to extract their published notes. Supports downloading cover
images and user avatars to local storage.
"""
import asyncio
import csv
import json
import os
import re
import time
import urllib.request
from pathlib import Path

from playwright.async_api import async_playwright

TASK_ID = os.getenv("SPIDER_LAB_TASK_ID", "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR = WORK_DIR / "output" / TASK_ID
IMAGE_DIR = WORK_DIR / "images"


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def parse_count(count_str):
    """Parse XHS count string like '8.1万' to integer."""
    if not count_str:
        return 0
    if isinstance(count_str, (int, float)):
        return int(count_str)
    s = str(count_str).strip()
    if "万" in s:
        try:
            return int(float(s.replace("万", "")) * 10000)
        except ValueError:
            return 0
    try:
        return int(s)
    except ValueError:
        return 0


def get_ext(url):
    for ext in (".png", ".gif", ".webp", ".jpeg"):
        if ext in url.lower():
            return ext
    return ".jpg"


def download_image(url, save_path):
    """Download image via direct HTTP request."""
    Path(save_path).parent.mkdir(parents=True, exist_ok=True)
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                          "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
            "Referer": "https://www.xiaohongshu.com/",
        })
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = resp.read()
            if len(data) > 0:
                with open(save_path, "wb") as f:
                    f.write(data)
                return True
    except Exception:
        pass
    return False


def download_cover(url, note_id, cache):
    """Download note cover image, return local path."""
    if not url or not url.startswith("http"):
        return ""
    if url in cache:
        return cache[url]
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    fname = f"cover_{note_id}{get_ext(url)}"
    save_path = str(IMAGE_DIR / fname)
    if download_image(url, save_path):
        cache[url] = save_path
        log(f"    Cover saved: {fname}")
        return save_path
    return ""


def download_avatar(url, user_id, cache):
    """Download user avatar, return local path."""
    if not url or not url.startswith("http"):
        return ""
    if url in cache:
        return cache[url]
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    fname = f"avatar_{user_id}{get_ext(url)}"
    save_path = str(IMAGE_DIR / fname)
    if download_image(url, save_path):
        cache[url] = save_path
        return save_path
    return ""


def save_results(data, config, user_name):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    ts = time.strftime("%Y%m%d_%H%M%S")
    safe_name = re.sub(r"[^\w]", "_", user_name)[:30]
    fmt = config.get("save_format", "json")
    if fmt == "csv" and data:
        filepath = OUTPUT_DIR / f"xhs_user_{safe_name}_{ts}.csv"
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
        filepath = OUTPUT_DIR / f"xhs_user_{safe_name}_{ts}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


async def main():
    config = load_config()
    user_id = str(config.get("user_id", "")).strip()
    max_notes = int(config.get("max_notes", 30))
    headless = config.get("headless", True)
    delay = float(config.get("request_delay", 2))
    do_images = config.get("download_images", True)

    if not user_id:
        log("ERROR: user_id is required. Set it in spider config.")
        return

    log(f"Spider started — user_id={user_id}, max_notes={max_notes}, images={do_images}")

    api_data = {"user": None, "notes": []}
    seen_note_ids = set()
    img_cache = {}

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=headless)
        context = await browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                       "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
            viewport={"width": 1440, "height": 900},
            locale="zh-CN",
        )

        async def on_response(response):
            url = response.url
            try:
                if "/api/sns/web/v1/user_posted" in url:
                    body = await response.json()
                    if body.get("success"):
                        notes = body.get("data", {}).get("notes", [])
                        for note in notes:
                            nid = note.get("note_id", "") or note.get("id", "")
                            if nid and nid not in seen_note_ids:
                                seen_note_ids.add(nid)
                                api_data["notes"].append(note)
                elif "/api/sns/web/v1/user/otherinfo" in url:
                    body = await response.json()
                    if body.get("success") and not api_data["user"]:
                        api_data["user"] = body.get("data", {})
            except Exception:
                pass

        page = await context.new_page()
        page.on("response", on_response)

        try:
            profile_url = f"https://www.xiaohongshu.com/user/profile/{user_id}"
            log(f"Visiting user profile: {profile_url}")
            await page.goto(profile_url, wait_until="networkidle", timeout=30000)
            await page.wait_for_timeout(5000)

            if not api_data["user"]:
                try:
                    state = await page.evaluate("() => window.__INITIAL_STATE__")
                    if state:
                        upd = state.get("user", {}).get("userPageData", {})
                        if upd:
                            api_data["user"] = upd
                except Exception:
                    pass

            prev_count = len(api_data["notes"])
            max_scroll_attempts = max(max_notes // 10 + 5, 10)
            for attempt in range(max_scroll_attempts):
                if len(api_data["notes"]) >= max_notes:
                    break
                await page.evaluate("window.scrollTo(0, document.body.scrollHeight)")
                await page.wait_for_timeout(int(delay * 1000))
                current_count = len(api_data["notes"])
                if current_count == prev_count:
                    await page.wait_for_timeout(2000)
                    if len(api_data["notes"]) == prev_count:
                        log(f"  No more notes loading (attempt {attempt+1})")
                        break
                prev_count = current_count
                log(f"  Loaded {current_count} notes...")

        except Exception as e:
            log(f"Error visiting profile: {e}")
        finally:
            await browser.close()

    # Process user info
    user_name = user_id
    avatar_url = ""
    local_avatar = ""
    if api_data["user"]:
        u = api_data["user"]
        basic = u.get("basicInfo", {})
        user_name = basic.get("nickname", "") or u.get("nickname", "") or user_id
        avatar_url = basic.get("imageb", "") or u.get("image", "")

        if do_images and avatar_url:
            local_avatar = download_avatar(avatar_url, user_id, img_cache)

        profile = {
            "type": "profile",
            "user_id": user_id,
            "user_name": user_name,
            "avatar_url": avatar_url,
            "local_avatar": local_avatar,
            "description": basic.get("desc", "") or u.get("desc", ""),
            "red_id": basic.get("redId", "") or u.get("redId", ""),
            "ip_location": basic.get("ipLocation", ""),
            "gender": basic.get("gender", 0),
            "task_id": TASK_ID,
            "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        }
        for item in u.get("interactions", []):
            itype = item.get("type", "")
            if itype == "fans":
                profile["fans_count"] = parse_count(item.get("count", 0))
            elif itype == "follows":
                profile["follows_count"] = parse_count(item.get("count", 0))
        tags = u.get("tags", [])
        profile["tags"] = [t.get("name", "") for t in tags if t.get("name")]

        print(json.dumps(profile, ensure_ascii=False), flush=True)
        log(f"User: {user_name}")

    # Process notes
    all_notes = []
    for note_data in api_data["notes"][:max_notes]:
        cover = note_data.get("cover", {})
        interact = note_data.get("interact_info", {})
        user = note_data.get("user", {})
        nid = note_data.get("note_id", "") or note_data.get("id", "")

        cover_url = cover.get("url", "") or cover.get("urlDefault", "")
        local_cover = ""
        if do_images and cover_url:
            local_cover = download_cover(cover_url, nid, img_cache)

        note = {
            "type": "note",
            "note_id": nid,
            "title": note_data.get("display_title", "") or note_data.get("title", ""),
            "note_type": note_data.get("type", ""),
            "user_id": user.get("user_id", "") or user_id,
            "user_name": user.get("nickname", "") or user_name,
            "avatar_url": avatar_url,
            "local_avatar": local_avatar,
            "liked_count": parse_count(interact.get("liked_count", 0)),
            "cover_url": cover_url,
            "local_cover": local_cover,
            "note_url": f"https://www.xiaohongshu.com/explore/{nid}",
            "rank": len(all_notes) + 1,
            "task_id": TASK_ID,
            "crawled_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        }
        all_notes.append(note)
        print(json.dumps(note, ensure_ascii=False), flush=True)

    img_count = len(img_cache)
    log(f"Collected {len(all_notes)} notes from {user_name}, {img_count} images downloaded")
    save_results(all_notes, config, user_name)
    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const xhsUserPostsReadme = `# 小红书用户笔记爬虫 (XHS User Posts)

## 功能
- 抓取指定小红书用户的全部/近期笔记
- 使用 Playwright 浏览器自动化访问用户主页
- 通过拦截 API 响应获取结构化笔记数据
- 自动下载笔记封面图和用户头像到本地
- 自动滚动加载更多笔记
- 提取用户资料（昵称、头像、粉丝数、简介等）
- 输出 JSON / CSV 格式

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| user_id | string | "" | 小红书用户ID（必填） |
| max_notes | number | 30 | 最大笔记抓取条数 |
| download_images | boolean | true | 下载图片到本地 |
| headless | boolean | true | 无头浏览器模式 |
| request_delay | number | 2 | 滚动加载间隔(秒) |
| save_format | string | "json" | 输出格式 json/csv |

## 图片下载
开启 download_images 后，图片保存在 output/images/ 目录：
- 笔记封面: cover_{note_id}.jpg
- 用户头像: avatar_{user_id}.jpg
结果数据中 local_cover / local_avatar 字段为本地路径。

## 如何获取用户ID
1. 打开小红书用户主页
2. URL 中 profile/ 后面的字符串即为用户ID
3. 例如：xiaohongshu.com/user/profile/5fa7492e00000000010049ef
4. 用户ID为 5fa7492e00000000010049ef

## 输出字段
- type: profile（用户资料）/ note（笔记）
- 用户资料：user_name, avatar, local_avatar, description, fans_count, follows_count
- 笔记：note_id, title, note_type, liked_count, cover_url, local_cover, note_url

## 依赖
- playwright (浏览器自动化 + API 拦截)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium

## 输出
数据保存在 output/ 目录，图片保存在 output/images/ 目录。
每条记录也以 JSON line 输出到 stdout。
`
