#!/usr/bin/env python3
"""Look up a Xueqiu user by ID.

Usage: python3 xueqiu_user_info.py <user_id>
Output: JSON object with user info to stdout.

Reuses the cookie cache from xueqiu_user_search.py so lookups after
a recent search are near-instant (~200ms). Falls back to Playwright
if cookies are stale.
"""

import json
import os
import sys
import time

COOKIE_CACHE = "/tmp/xueqiu_cookies.json"
CACHE_TTL = 3600  # 1 hour


def load_cached_cookies():
    try:
        if not os.path.exists(COOKIE_CACHE):
            return None
        stat = os.stat(COOKIE_CACHE)
        if time.time() - stat.st_mtime > CACHE_TTL:
            return None
        with open(COOKIE_CACHE) as f:
            cookies = json.load(f)
        if "xq_a_token" not in cookies:
            return None
        return cookies
    except Exception:
        return None


def save_cookies(cookies: dict):
    try:
        with open(COOKIE_CACHE, "w") as f:
            json.dump(cookies, f)
    except Exception:
        pass


def lookup_via_http(user_id: str, cookies: dict):
    """Fetch user/show.json with cached cookies."""
    import urllib.request

    url = f"https://xueqiu.com/user/show.json?id={user_id}"
    cookie_str = "; ".join(f"{k}={v}" for k, v in cookies.items())
    req = urllib.request.Request(url, headers={
        "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                      "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
        "Accept": "application/json, text/plain, */*",
        "X-Requested-With": "XMLHttpRequest",
        "Origin": "https://xueqiu.com",
        "Referer": "https://xueqiu.com/",
        "Cookie": cookie_str,
    })
    with urllib.request.urlopen(req, timeout=10) as resp:
        ct = resp.headers.get("Content-Type", "")
        body = resp.read()
        if "json" not in ct:
            return None  # WAF challenge
        data = json.loads(body)
        if data.get("error_code"):
            return None
        return data


def lookup_via_playwright(user_id: str):
    """Launch Playwright, solve WAF, fetch user info, cache cookies."""
    from playwright.sync_api import sync_playwright
    from playwright_stealth import Stealth

    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
                  "--disable-blink-features=AutomationControlled"],
        )
        ctx = browser.new_context(
            user_agent="Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                       "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
            viewport={"width": 1440, "height": 900},
            locale="zh-CN",
        )
        page = ctx.new_page()
        Stealth().apply_stealth_sync(page)
        page.goto("https://xueqiu.com/", wait_until="domcontentloaded", timeout=30000)

        cookies = {}
        for _ in range(15):
            cookies_list = ctx.cookies()
            cookies = {c["name"]: c["value"] for c in cookies_list}
            if "xq_a_token" in cookies:
                break
            time.sleep(1)

        if "xq_a_token" not in cookies:
            browser.close()
            return None

        save_cookies(cookies)

        result = page.evaluate(
            """async (uid) => {
            try {
                const resp = await fetch(
                    `https://xueqiu.com/user/show.json?id=${uid}`,
                    { credentials: 'include',
                      headers: { 'X-Requested-With': 'XMLHttpRequest' } }
                );
                const ct = resp.headers.get('content-type') || '';
                if (!ct.includes('json')) return null;
                const data = await resp.json();
                if (data.error_code) return null;
                return data;
            } catch { return null; }
        }""",
            user_id,
        )

        browser.close()
        return result


def main():
    if len(sys.argv) < 2:
        print("{}", flush=True)
        return

    user_id = sys.argv[1].strip()
    if not user_id:
        print("{}", flush=True)
        return

    # Fast path: cached cookies
    cached = load_cached_cookies()
    if cached:
        try:
            result = lookup_via_http(user_id, cached)
            if result:
                print(json.dumps(result, ensure_ascii=False))
                return
        except Exception:
            pass

    # Slow path: Playwright
    try:
        result = lookup_via_playwright(user_id)
        if result:
            print(json.dumps(result, ensure_ascii=False))
            return
    except Exception:
        pass

    print("{}", flush=True)


if __name__ == "__main__":
    main()
