package spider

// ==================== WeChat Public Account ====================

func wechatUserArticlesTemplate() *SpiderTemplate {
	minArticles := float64(1)
	maxArticles := float64(2000)
	minDelay := float64(0)
	maxDelay := float64(30)

	return &SpiderTemplate{
		Id:          "wechat_user_articles",
		Name:        "微信公众号文章",
		Platform:    "wechat",
		Description: "抓取指定微信公众号的历史文章列表，包括标题、作者、发布日期、原文链接。可选抓取每篇文章的正文内容和图片。需要提供公众号管理后台（mp.weixin.qq.com）的登录 Cookie。支持按数量、全部抓取、按起始日期三种模式。",
		Icon:        "message-circle",
		Tags:        []string{"微信", "公众号", "文章", "Playwright"},
		Cmd:         "python3 main.py",
		ConfigFields: []ConfigField{
			{Key: "target_account_id", Label: "目标公众号", Type: "select", Default: "", Required: false, Description: "从目标账号池中选择公众号（自动填充公众号名称和fakeid）", Options: []string{}, OptionLabels: []string{}},
			{Key: "account_name", Label: "公众号名称", Type: "string", Default: "", Required: false, Description: "目标公众号的名称（用于在管理后台搜索 fakeid，也作为显示名）", Placeholder: "人民日报"},
			{Key: "biz", Label: "公众号Biz/Fakeid", Type: "string", Default: "", Required: false, Description: "公众号的 fakeid（可选，填入后跳过搜索步骤，提高可靠性）。从 searchbiz API 获取，或在管理后台文章列表 URL 中的 fakeid 参数", Placeholder: "MjM5MjQ4OTc2MA=="},
			{Key: "scrape_mode", Label: "抓取模式", Type: "select", Default: "count", Required: false, Description: "按数量抓取（count）、全部抓取（all）、或从指定日期起抓取（since_date）", Options: []string{"count", "all", "since_date"}, OptionLabels: []string{"按数量", "全部", "按日期"}},
			{Key: "max_articles", Label: "最大文章数", Type: "number", Default: 50, Required: false, Description: "最多抓取的文章数（仅「按数量」模式生效）", Min: &minArticles, Max: &maxArticles},
			{Key: "since_date", Label: "起始日期", Type: "string", Default: "", Required: false, Description: "从该日期（含）起抓取文章（格式 YYYY-MM-DD，仅「按日期」模式生效）", Placeholder: "2025-01-01"},
			{Key: "fetch_content", Label: "抓取正文", Type: "boolean", Default: false, Required: false, Description: "是否抓取每篇文章的完整正文（通过 httpx 直接请求公开文章页面，无需额外登录）"},
			{Key: "download_images", Label: "下载图片", Type: "boolean", Default: true, Required: false, Description: "是否下载文章中的图片到本地（仅在启用抓取正文时生效）"},
			{Key: "headless", Label: "无头模式", Type: "boolean", Default: true, Required: false, Description: "是否使用无头浏览器"},
			{Key: "request_delay", Label: "请求间隔(秒)", Type: "number", Default: 2, Required: false, Description: "API 请求之间的等待时间", Min: &minDelay, Max: &maxDelay},
			{Key: "save_format", Label: "输出格式", Type: "select", Default: "json", Required: false, Description: "数据保存格式", Options: []string{"json", "csv"}, OptionLabels: []string{"JSON", "CSV"}},
		},
		Files: map[string]string{
			"main.py":          wechatUserArticlesMain,
			"requirements.txt": wechatRequirements,
			"README.md":        wechatUserArticlesReadme,
		},
	}
}

const wechatRequirements = `playwright>=1.40.0
httpx>=0.25.0
beautifulsoup4>=4.12.0
lxml>=4.9.0
`

const wechatUserArticlesMain = `#!/usr/bin/env python3
"""
Spider Lab - 微信公众号文章爬虫
WeChat Public Account Article Spider

Uses Playwright to authenticate with WeChat MP (mp.weixin.qq.com),
then paginates through the account's article list via the internal
appmsg list_ex API. Optionally fetches full article HTML via httpx.

Authentication flow:
  1. Inject mp_cookie into browser context
  2. Navigate to /cgi-bin/home — redirected URL contains the session token
  3. Use appmsg list_ex to list articles (requires token + fakeid/biz)
  4. If biz not provided, auto-search via searchbiz API

Scrape modes:
  count      — collect up to max_articles articles
  all        — collect every available article
  since_date — collect articles published on or after since_date (YYYY-MM-DD)
"""
import asyncio
import base64
import csv
import json
import os
import re
import sys
import time
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import parse_qs, urlparse

from playwright.async_api import async_playwright

TASK_ID   = os.getenv("SPIDER_LAB_TASK_ID",   "unknown")
SPIDER_ID = os.getenv("SPIDER_LAB_SPIDER_ID", "unknown")
WORK_DIR    = Path(__file__).parent
CONFIG_FILE = WORK_DIR / "config.json"
OUTPUT_DIR  = WORK_DIR / "output" / TASK_ID
IMAGE_DIR   = WORK_DIR / "images"

MP_API = "https://mp.weixin.qq.com/cgi-bin"
MP_URL = "https://mp.weixin.qq.com"

DESKTOP_UA = (
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
    "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)


# ─────────────────── helpers ──────────────────────────────────────

def log(msg):
    print(f"[{TASK_ID}] {msg}", flush=True)


def load_config():
    if CONFIG_FILE.exists():
        with open(CONFIG_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    return {}


def parse_mp_cookies(cookie_str):
    """Parse semicolon-separated cookie string into Playwright dicts."""
    cookies = []
    for item in (cookie_str or "").split(";"):
        item = item.strip()
        if "=" in item:
            k, v = item.split("=", 1)
            cookies.append({
                "name":   k.strip(),
                "value":  v.strip(),
                "domain": "mp.weixin.qq.com",
                "path":   "/",
            })
    return cookies


def save_results(data, account_name, fmt):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    ts = time.strftime("%Y%m%d_%H%M%S")
    safe = re.sub(r"[^\w]", "_", account_name)[:30]
    if fmt == "csv" and data:
        filepath = OUTPUT_DIR / f"wechat_{safe}_{ts}.csv"
        flat = []
        for item in data:
            row = {k: v for k, v in item.items()
                   if not isinstance(v, (dict, list)) and k != "html_content"}
            row["plain_text_preview"] = (item.get("plain_text") or "")[:500]
            flat.append(row)
        keys = list(flat[0].keys()) if flat else []
        with open(filepath, "w", newline="", encoding="utf-8-sig") as f:
            w = csv.DictWriter(f, fieldnames=keys)
            w.writeheader()
            w.writerows(flat)
    else:
        filepath = OUTPUT_DIR / f"wechat_{safe}_{ts}.json"
        with open(filepath, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)
    log(f"Results saved to {filepath}")


def get_ext(url):
    for ext in (".png", ".gif", ".webp", ".jpeg"):
        if ext in url.lower():
            return ext
    return ".jpg"


async def download_image(url, save_path):
    Path(save_path).parent.mkdir(parents=True, exist_ok=True)
    try:
        req = urllib.request.Request(url, headers={
            "User-Agent": DESKTOP_UA,
            "Referer": "https://mp.weixin.qq.com/",
        })
        with urllib.request.urlopen(req, timeout=15) as resp:
            data = resp.read()
            if len(data) > 0:
                with open(save_path, "wb") as f:
                    f.write(data)
                return True
    except Exception as e:
        log(f"    Image download failed (urllib): {e}")
    return False


def extract_image_urls(html_text):
    if not html_text:
        return []
    img_re = re.compile(r'<img[^>]+src=["\x27]([^"\x27]+)["\x27]', re.IGNORECASE)
    urls = [u for u in img_re.findall(html_text) if u.startswith("http")]
    # Also check data-src attributes (WeChat uses lazy loading)
    data_src_re = re.compile(r'<img[^>]+data-src=["\x27]([^"\x27]+)["\x27]', re.IGNORECASE)
    data_urls = [u for u in data_src_re.findall(html_text) if u.startswith("http")]
    return list(set(urls + data_urls))


async def download_article_images(image_urls, article_idx, cache):
    IMAGE_DIR.mkdir(parents=True, exist_ok=True)
    results = []
    for i, url in enumerate(image_urls[:20]):
        if url in cache:
            results.append(cache[url])
            continue
        fname = f"article_{article_idx}_{i}{get_ext(url)}"
        save_path = str(IMAGE_DIR / fname)
        api_url = f"/api/spiders/{SPIDER_ID}/images/{fname}"
        if await download_image(url, save_path):
            entry = (save_path, api_url)
            results.append(entry)
            cache[url] = entry
            log(f"    Downloaded image {i+1}/{len(image_urls)}")
    return results


# ─────────────────── API helpers ──────────────────────────────────

async def api_get(page, url, max_retries=3):
    """Fetch a JSON endpoint through the browser (carries MP session cookies)."""
    for attempt in range(max_retries):
        try:
            result = await page.evaluate(
                """async (url) => {
                    try {
                        const resp = await fetch(url, {
                            credentials: 'include',
                            headers: {
                                'Accept': 'application/json, text/javascript, */*',
                                'X-Requested-With': 'XMLHttpRequest',
                            },
                        });
                        const text = await resp.text();
                        if (!resp.ok)
                            return {__error: true, status: resp.status, text: text.slice(0,300)};
                        try { return JSON.parse(text); }
                        catch { return {__error: true, status: 0, text: text.slice(0,300)}; }
                    } catch(e) {
                        return {__error: true, status: -1, text: String(e)};
                    }
                }""", url)
            if isinstance(result, dict) and result.get("__error"):
                log(f"  API error status={result.get('status')} text={result.get('text','')[:80]} (attempt {attempt+1})")
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


async def get_token_from_page(page):
    """Extract the token query param from the current page URL."""
    m = re.search(r"[?&]token=(\d+)", page.url)
    return m.group(1) if m else None


async def resolve_token(page, token_from_config):
    """Return a valid MP token: use config value if provided, else extract from URL."""
    if token_from_config:
        return token_from_config
    token = await get_token_from_page(page)
    if token:
        return token
    # Try navigating to appmsg page to refresh token in URL
    try:
        await page.goto(f"{MP_URL}/cgi-bin/appmsg",
                        wait_until="domcontentloaded", timeout=15000)
        await page.wait_for_timeout(2000)
    except Exception:
        pass
    return await get_token_from_page(page)


async def search_biz(page, token, account_name):
    """Search for a public account's fakeid by display name."""
    from urllib.parse import quote
    url = (
        f"{MP_API}/searchbiz"
        f"?action=search_biz&begin=0&count=5"
        f"&query={quote(account_name)}&token={token}&lang=zh_CN&f=json&ajax=1"
    )
    data = await api_get(page, url)
    if not data or data.get("base_resp", {}).get("ret") != 0:
        log(f"  searchbiz failed: ret={data.get('base_resp',{}).get('ret') if data else 'None'}")
        return ""
    items = data.get("list", [])
    if not items:
        log(f"  No accounts found for '{account_name}'")
        return ""
    fakeid   = items[0].get("fakeid",   "")
    nickname = items[0].get("nickname", "")
    log(f"  Found account: {nickname} (fakeid={fakeid})")
    return fakeid


# ─────────────────── content scraping ─────────────────────────────

async def fetch_article_content(article_url, download_images_flag, article_idx, img_cache):
    """Fetch a WeChat article's full content via httpx (public pages, no auth)."""
    try:
        import httpx
        from bs4 import BeautifulSoup
        headers = {
            "User-Agent": DESKTOP_UA,
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
            "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
            "Referer": "https://mp.weixin.qq.com/",
            "Cache-Control": "no-cache",
        }
        async with httpx.AsyncClient(timeout=30, follow_redirects=True) as client:
            resp = await client.get(article_url, headers=headers)
        if resp.status_code != 200:
            log(f"    HTTP {resp.status_code} for {article_url[:80]}")
            return None, None, []
        soup = BeautifulSoup(resp.text, "html.parser")
        content_el = (
            soup.find(id="js_content") or
            soup.find(id="js_article") or
            soup.find(class_="rich_media_content")
        )
        if not content_el:
            return None, None, []
        
        # Extract image URLs before modifying HTML
        image_urls = []
        if download_images_flag:
            image_urls = extract_image_urls(str(content_el))
        
        # Fix lazy-loaded images: data-src → src
        for img in content_el.find_all("img"):
            data_src = img.get("data-src")
            if data_src and not img.get("src"):
                img["src"] = data_src
        
        html_content = content_el.decode_contents()
        plain_text = content_el.get_text(separator="\\n", strip=True)
        
        # Download images if requested
        local_images = []
        api_images = []
        if download_images_flag and image_urls:
            dl_results = await download_article_images(image_urls, article_idx, img_cache)
            local_images = [r[0] for r in dl_results]
            api_images = [r[1] for r in dl_results]
            # Replace original image URLs in HTML with local API URLs
            for original_url, api_url in zip(image_urls, api_images):
                html_content = html_content.replace(original_url, api_url)
        
        return html_content, plain_text, api_images
    except ImportError:
        log("  httpx / beautifulsoup4 not installed — skipping content fetch")
        log("  Run: pip install httpx beautifulsoup4 lxml")
        return None, None, []
    except Exception as e:
        log(f"  Content fetch error for {article_url[:80]}: {e}")
        return None, None, []


# ─────────────────── article collection ───────────────────────────

async def collect_links(page, token, fakeid, account_name,
                        scrape_mode, max_articles, since_date, delay,
                        fetch_content_flag, download_images_flag, img_cache):
    """Paginate through appmsg list_ex to collect article metadata."""
    all_articles = []
    begin        = 0
    page_size    = 20
    total_count  = None
    stop_crawl   = False

    while not stop_crawl:
        if scrape_mode == "count":
            remaining = max_articles - len(all_articles)
            if remaining <= 0:
                break
            count = min(page_size, remaining)
        else:
            count = page_size

        url = (
            f"{MP_API}/appmsg"
            f"?action=list_ex&begin={begin}&count={count}"
            f"&fakeid={fakeid}&type=9&token={token}&lang=zh_CN"
        )
        data = await api_get(page, url)
        if not data:
            log(f"  appmsg list_ex returned None at begin={begin}")
            break
        ret = data.get("base_resp", {}).get("ret", -1)
        if ret != 0:
            log(f"  appmsg list_ex ret={ret} at begin={begin} "
                f"(ret=200004 means token expired; re-login via Account Management)")
            break

        if total_count is None:
            total_count = data.get("app_msg_cnt", 0)
            log(f"  Total articles for '{account_name}': {total_count}")

        items = data.get("app_msg_list", [])
        if not items:
            break

        for item in items:
            link        = item.get("link",   "")
            title       = item.get("title",  "")
            author      = item.get("author", account_name)
            create_time = item.get("create_time", 0)
            pub_date    = (
                datetime.fromtimestamp(create_time, tz=timezone.utc)
                         .strftime("%Y-%m-%d")
                if create_time else ""
            )

            # since_date mode: stop when article is older than cutoff
            if scrape_mode == "since_date" and since_date and pub_date:
                if pub_date < since_date:
                    log(f"  Reached article older than {since_date} ({pub_date}), stopping")
                    stop_crawl = True
                    break

            # idx=1 in the URL means this is the headline article of the push
            try:
                url_idx = int(
                    (parse_qs(urlparse(link).query).get("idx") or ["1"])[0]
                )
            except (ValueError, IndexError):
                url_idx = 1

            article = {
                "title":          title,
                "author":         author,
                "url":            link,
                "publish_date":   pub_date,
                "is_headline":    url_idx == 1,
                "source_account": account_name,
                "biz":            fakeid,
                "task_id":        TASK_ID,
                "crawled_at":     time.strftime("%Y-%m-%d %H:%M:%S"),
            }

            # Publisher sessions expose read/like metrics
            read_num = item.get("read_num")
            like_num = item.get("like_num")
            if read_num is not None:
                article["read_count"] = int(read_num)
            if like_num is not None:
                article["like_count"] = int(like_num)

            # Optionally fetch full article content via httpx (no auth needed)
            if fetch_content_flag:
                html_content, plain_text, images = await fetch_article_content(
                    link, download_images_flag, len(all_articles), img_cache)
                if plain_text:
                    article["html_content"] = html_content
                    article["plain_text"] = plain_text
                    article["images"] = images
                    log(f"    {len(images)} images processed")

            all_articles.append(article)
            log(f"  [{len(all_articles)}] {pub_date}  {title[:60]}")

            if scrape_mode == "count" and len(all_articles) >= max_articles:
                stop_crawl = True
                break

        if stop_crawl:
            break

        begin += len(items)
        if begin >= (total_count or 0):
            break

        await asyncio.sleep(delay)

    log(f"Collected {len(all_articles)} article links")
    return all_articles


# ─────────────────── main ─────────────────────────────────────────

async def main():
    config        = load_config()
    account_name  = config.get("account_name",  "").strip()
    biz           = config.get("biz",            "").strip()
    cookie        = config.get("cookie",  "").strip()  # injected by platform from cookie pool
    token_cfg     = config.get("token",   "").strip()  # injected by platform (MP token)
    scrape_mode   = config.get("scrape_mode",    "count")
    max_articles  = int(config.get("max_articles", 50))
    since_date    = config.get("since_date",     "").strip()
    fetch_content = config.get("fetch_content",  False)
    download_images = config.get("download_images", True)
    request_delay = float(config.get("request_delay", 2))
    save_format   = config.get("save_format",    "json")
    headless      = config.get("headless",       True)

    # Validation
    if not account_name:
        log("ERROR: account_name is required.")
        sys.exit(1)
    if not cookie:
        log("ERROR: No WeChat MP cookie available.")
        log("  Please add a WeChat public-account worker to the Cookie Pool")
        log("  in the Account Management page (账号管理 → 工作账号 → 微信).")
        sys.exit(1)
    if scrape_mode == "since_date" and not since_date:
        log("WARNING: scrape_mode=since_date but since_date is empty; switching to 'all'")
        scrape_mode = "all"

    mode_desc = (f"max={max_articles}"  if scrape_mode == "count"
                 else f"since={since_date}" if scrape_mode == "since_date"
                 else "all")
    log(f"Spider started — account={account_name}, mode={scrape_mode}({mode_desc}), "
        f"fetch_content={fetch_content}, download_images={download_images}")

    # Launch browser
    pw      = await async_playwright().start()
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

    cookies = parse_mp_cookies(cookie)
    if not cookies:
        log("ERROR: Could not parse injected cookie string.")
        await browser.close()
        await pw.stop()
        sys.exit(1)

    await context.add_cookies(cookies)
    log(f"Injected {len(cookies)} MP cookies")

    page = await context.new_page()

    # Image cache
    img_cache = {}

    try:
        # Navigate to MP home — this validates the session and puts token in URL
        log("Navigating to mp.weixin.qq.com/cgi-bin/home ...")
        try:
            await page.goto(f"{MP_URL}/cgi-bin/home",
                            wait_until="domcontentloaded", timeout=30000)
            await page.wait_for_timeout(3000)
        except Exception as e:
            log(f"  Navigation warning: {e}")

        current_url = page.url
        log(f"  Current URL: {current_url[:120]}")

        if ("loginpage" in current_url
                or current_url.rstrip("/") in (MP_URL.rstrip("/"), "")):
            log("ERROR: Session invalid — not logged in to mp.weixin.qq.com.")
            log("  The cookie injected from the pool may have expired.")
            log("  Please re-login via Account Management (账号管理 → 工作账号 → 微信).")
            sys.exit(1)

        # Extract token
        token = await resolve_token(page, token_cfg)
        if not token:
            log("ERROR: Could not obtain MP token. Ensure the cookie is valid "
                "and you are logged in to mp.weixin.qq.com.")
            sys.exit(1)
        log(f"  Token: {token}")

        # Find fakeid (biz) if not provided
        if not biz:
            log(f"  Searching for account '{account_name}'...")
            biz = await search_biz(page, token, account_name)
            if not biz:
                log(f"ERROR: Could not find account '{account_name}'. "
                    "Set 'biz' (fakeid) directly in the spider config to skip search.")
                sys.exit(1)

        # Collect article metadata
        all_articles = await collect_links(
            page, token, biz, account_name,
            scrape_mode, max_articles, since_date,
            request_delay, fetch_content, download_images, img_cache,
        )

        # Print results to stdout (captured by Spider Lab runner)
        for article in all_articles:
            print(json.dumps(article, ensure_ascii=False), flush=True)

        log(f"Total: {len(all_articles)} articles, {len(img_cache)} images downloaded")
        save_results(all_articles, account_name, save_format)

    finally:
        await browser.close()
        await pw.stop()

    log("Spider completed")


if __name__ == "__main__":
    asyncio.run(main())
`

const wechatUserArticlesReadme = `# 微信公众号文章爬虫 (WeChat Public Account)

## 功能
- 抓取微信公众号历史文章列表（标题、作者、发布日期、链接）
- 支持三种抓取模式：按数量、全部抓取、按起始日期
- 可选抓取每篇文章完整正文（无需额外登录，通过公开 URL 获取）
- 自动搜索公众号 fakeid（也可手动提供提高可靠性）
- 通过管理后台内部 appmsg API 获取文章列表
- 输出 JSON / CSV 格式

## 认证说明

本爬虫需要微信公众号管理后台（mp.weixin.qq.com）的登录 Cookie。

### 如何获取 Cookie
1. 在浏览器中登录 [mp.weixin.qq.com](https://mp.weixin.qq.com)
2. 打开浏览器开发者工具（F12）→ Network 标签
3. 刷新页面，点击任意请求
4. 在 Headers → Request Headers 中找到 ` + "`Cookie`" + ` 字段
5. 复制整个 Cookie 字符串粘贴到 ` + "`mp_cookie`" + ` 配置项

### Cookie 有效期
微信 MP Cookie 通常有效期为数天到数周。Cookie 失效后（ret=200004），
请重新登录并更新 mp_cookie 配置。

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| account_name | string | (必填) | 公众号名称（用于搜索 fakeid） |
| biz | string | "" | 公众号 fakeid（可选，填入可跳过搜索步骤） |
| mp_cookie | string | (必填) | mp.weixin.qq.com Cookie 字符串 |
| scrape_mode | select | "count" | 抓取模式：count/all/since_date |
| max_articles | number | 50 | 最大文章数（count 模式生效） |
| since_date | string | "" | 起始日期 YYYY-MM-DD（since_date 模式生效） |
| fetch_content | boolean | false | 是否抓取文章正文 |
| headless | boolean | true | 无头浏览器模式 |
| request_delay | number | 2 | 请求间隔(秒) |
| save_format | string | "json" | 输出格式 json/csv |

## 抓取模式
- **count** — 抓取最新 N 篇文章（max_articles 控制数量）
- **all** — 抓取公众号所有历史文章
- **since_date** — 抓取指定日期（含）之后发布的全部文章，如 since_date=2025-01-01

## 如何获取公众号 biz/fakeid（可选）
填写 biz 后可跳过搜索步骤，提高可靠性：
1. 在管理后台打开目标公众号的文章列表页面
2. 观察 URL 中的 ` + "`fakeid=xxx`" + ` 参数
3. 将该值填入 biz 配置项

## 正文抓取说明
开启 fetch_content 后，每篇文章的公开 URL 会通过 httpx 直接请求，
提取 #js_content 中的正文 HTML 和纯文本。
此操作无需登录，但需要额外安装 httpx + beautifulsoup4：

    pip install httpx beautifulsoup4 lxml

## 输出字段
- title — 文章标题
- author — 作者
- url — 原文链接
- publish_date — 发布日期（YYYY-MM-DD）
- is_headline — 是否为推送头条
- source_account — 公众号名称
- biz — 公众号 fakeid
- read_count — 阅读量（仅用自己的账号登录时可见）
- like_count — 点赞数（同上）
- plain_text — 文章正文纯文本（开启 fetch_content 后）
- html_content — 文章正文 HTML（保存到文件，stdout 输出省略）

## 依赖
- playwright (浏览器自动化 + MP 登录会话)
- httpx (文章正文抓取，仅 fetch_content=true 时需要)
- beautifulsoup4 + lxml (HTML 解析，同上)

## 首次运行
Worker 节点需要安装 Playwright 浏览器：

    playwright install chromium

## 输出
数据保存在 output/ 目录，每条文章记录以 JSON line 输出到 stdout。
`
