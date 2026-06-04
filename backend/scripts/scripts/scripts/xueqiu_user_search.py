#!/usr/bin/env python3
"""Xueqiu user search via Playwright — bypasses Aliyun WAF.

Usage: python3 xueqiu_user_search.py <query> [count]
Output: JSON array of user objects to stdout.

Caches browser cookies to /tmp/xueqiu_cookies.json so only the first
search incurs the ~3s Playwright startup; subsequent searches within
the cache TTL use plain HTTP requests.
"""

import json
import os
import sys
import time

COOKIE_CACHE = "/tmp/xueqiu_cookies.json"
CACHE_TTL = 3600  # 1 hour


def load_cached_cookies():
    """Load cached cookies if fresh enough."""
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
    """Save cookies to cache file."""
    try:
        with open(COOKIE_CACHE, "w") as f:
            json.dump(cookies, f)
    except Exception:
        pass


def search_via_http(query: str, count: int, cookies: dict) -> list | None:
    """Try searching with cached cookies via plain HTTP."""
    import requests

    try:
        resp = requests.get(
            "https://xueqiu.com/query/v1/search/user.json",
            params={"q": query, "count": str(count), "page": "1"},
            headers={
                "User-Agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
                "Accept": "application/json",
                "Referer": "https://xueqiu.com/",
            },
            cookies=cookies,
            timeout=10,
        )
        if resp.status_code != 200:
            return None
        ct = resp.headers.get("Content-Type", "")
        if "json" not in ct:
            return None
        data = resp.json()
        if "list" in data:
            return data["list"]
        if data.get("error_code"):
            return None  # cookie expired
        return []
    except Exception:
        return None


def search_via_playwright(query: str, count: int) -> list:
    """Launch Playwright, solve WAF, search, cache cookies."""
    from playwright.sync_api import sync_playwright
    from playwright_stealth import Stealth

    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=["--no-sandbox", "--disable-gpu", "--disable-dev-shm-usage",
                  "--disable-blink-features=AutomationControlled"],
        )
        page = browser.new_page()
        Stealth().apply_stealth_sync(page)

        page.goto("https://xueqiu.com/", wait_until="domcontentloaded", timeout=30000)

        # Wait for xq_a_token cookie (WAF challenge auto-solved by browser JS)
        for _ in range(15):
            cookies_list = page.context.cookies()
            cookies = {c["name"]: c["value"] for c in cookies_list}
            if "xq_a_token" in cookies:
                break
            time.sleep(1)

        if "xq_a_token" not in cookies:
            browser.close()
            return []

        # Cache cookies for future HTTP-only searches
        save_cookies(cookies)

        # Search users via fetch()
        result = page.evaluate(
            """async ([q, cnt]) => {
            try {
                const resp = await fetch(`/query/v1/search/user.json?q=${encodeURIComponent(q)}&count=${cnt}&page=1`);
                const ct = resp.headers.get("content-type") || "";
                if (!ct.includes("json")) return {error: "not json"};
                return await resp.json();
            } catch(e) {
                return {error: e.message};
            }
        }""",
            [query, count],
        )

        browser.close()

        if isinstance(result, dict) and "list" in result:
            return result["list"]
        return []


def main():
    query = sys.argv[1] if len(sys.argv) > 1 else ""
    count = int(sys.argv[2]) if len(sys.argv) > 2 else 8

    if not query:
        print("[]")
        return

    # Try cached cookies first (fast path ~200ms)
    cached = load_cached_cookies()
    if cached:
        users = search_via_http(query, count, cached)
        if users is not None:
            print(json.dumps(users, ensure_ascii=False))
            return

    # Fall back to Playwright (slow path ~3-5s)
    users = search_via_playwright(query, count)
    print(json.dumps(users, ensure_ascii=False))


if __name__ == "__main__":
    main()
