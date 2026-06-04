#!/usr/bin/env python3
"""Weibo user search via s.weibo.com desktop search page.

Usage: python3 weibo_user_search.py <query> [count]
Output: JSON array of user objects to stdout.

Uses Playwright to navigate to s.weibo.com/user and scrapes user cards
from the DOM.  The mobile m.weibo.cn API now triggers CAPTCHA for
headless browsers, so we scrape the desktop search page instead.
"""

import json
import sys
import time
import urllib.parse

from playwright.sync_api import sync_playwright
from playwright_stealth import Stealth

JS_EXTRACT_USERS = r"""(count) => {
    const results = [];
    const cards = document.querySelectorAll('.card.card-user-b, .card-wrap[class*="card-user"]');
    for (const card of cards) {
        if (results.length >= count) break;

        const nameEl = card.querySelector('.name, .m-text-cut');
        const name = nameEl ? nameEl.textContent.trim() : '';
        if (!name) continue;

        // uid from profile link (/u/1234567890)
        const link = card.querySelector('a[href*="/u/"]');
        let uid = '';
        if (link) {
            const m = (link.getAttribute('href') || '').match(/\/u\/(\d+)/);
            if (m) uid = m[1];
        }
        if (!uid) continue;

        // avatar
        const img = card.querySelector('.avator img, .photo img, .face img, img');
        const avatar = img ? img.src : '';

        // description — the <p> or <span> after the name inside .info
        let desc = '';
        const infoP = card.querySelector('.info p, .info .s-nobr');
        if (infoP) desc = infoP.textContent.trim();

        // followers text like "粉丝：1234.5万"
        let followers_count = 0;
        const fansMatch = card.textContent.match(/粉丝[：:]\s*([\d.]+)([万亿])?/);
        if (fansMatch) {
            let n = parseFloat(fansMatch[1]);
            if (fansMatch[2] === '万') n *= 10000;
            else if (fansMatch[2] === '亿') n *= 100000000;
            followers_count = Math.round(n);
        }

        // verified — yellow V or blue V icon
        const hasVIcon = !!card.querySelector(
            'i[class*="icon_yellow"], i[class*="icon_blue"], ' +
            'svg[class*="yellow"], svg[class*="blue"], ' +
            '.icon-approve, .icon_approve, [title*="认证"]'
        );

        let verified_reason = '';
        const vrEl = card.querySelector('.info p:nth-of-type(1)');
        if (vrEl && hasVIcon) verified_reason = vrEl.textContent.trim();

        results.push({
            id: parseInt(uid, 10) || uid,
            screen_name: name,
            profile_image_url: avatar,
            avatar_hd: avatar,
            description: desc,
            followers_count: followers_count,
            verified: hasVIcon,
            verified_reason: verified_reason,
            verified_type: hasVIcon ? 0 : -1,
        });
    }
    return results;
}"""


def search(query: str, count: int) -> list:
    """Search Weibo users via s.weibo.com and scrape the DOM."""
    with sync_playwright() as p:
        browser = p.chromium.launch(
            headless=True,
            args=[
                "--no-sandbox",
                "--disable-gpu",
                "--disable-dev-shm-usage",
                "--disable-blink-features=AutomationControlled",
            ],
        )
        ctx = browser.new_context(
            user_agent=(
                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36"
            ),
            viewport={"width": 1440, "height": 900},
            locale="zh-CN",
        )
        page = ctx.new_page()
        Stealth().apply_stealth_sync(page)

        url = (
            "https://s.weibo.com/user?q="
            + urllib.parse.quote(query)
            + "&Refer=weibo_user"
        )
        page.goto(url, wait_until="domcontentloaded", timeout=30000)
        time.sleep(4)

        users = page.evaluate(JS_EXTRACT_USERS, count)

        browser.close()
        return users if users else []


def main():
    query = sys.argv[1] if len(sys.argv) > 1 else ""
    count = int(sys.argv[2]) if len(sys.argv) > 2 else 8

    if not query:
        print("[]")
        return

    users = search(query, count)
    print(json.dumps(users, ensure_ascii=False))


if __name__ == "__main__":
    main()
