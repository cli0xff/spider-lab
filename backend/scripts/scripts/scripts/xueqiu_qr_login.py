#!/usr/bin/env python3
"""
Xueqiu QR Login Helper for Spider Lab Cookie Pool.

Launches a Playwright browser, navigates to Xueqiu, shows the QR login tab,
captures the QR code image, and waits for the user to scan.

Outputs JSON lines to stdout:
  {"type":"qr",      "image":"base64..."}       — QR code ready (PNG base64)
  {"type":"success",  "cookies":"...", "nickname":"...", "uid":"...", "avatar":"..."}
  {"type":"expired"}                             — QR code expired (2min timeout)
  {"type":"error",    "message":"..."}           — something went wrong

Usage: python3 xueqiu_qr_login.py [timeout_seconds]
"""
import asyncio
import base64
import json
import sys
import time

from playwright.async_api import async_playwright
from playwright_stealth import Stealth


def emit(data: dict):
    print(json.dumps(data, ensure_ascii=False), flush=True)


async def resolve_waf(page, ctx):
    """Visit homepage and wait for xq_a_token cookie (Aliyun WAF challenge)."""
    try:
        await page.goto(
            "https://xueqiu.com/",
            wait_until="domcontentloaded",
            timeout=60000,
        )
        for _ in range(30):
            cookies = await ctx.cookies()
            if any(c["name"] == "xq_a_token" for c in cookies):
                return True
            await page.wait_for_timeout(1000)
    except Exception:
        pass
    return False


async def main():
    timeout = int(sys.argv[1]) if len(sys.argv) > 1 else 120

    try:
        async with async_playwright() as p:
            browser = await p.chromium.launch(
                headless=True,
                args=[
                    "--no-sandbox",
                    "--disable-gpu",
                    "--disable-dev-shm-usage",
                    "--disable-blink-features=AutomationControlled",
                ],
            )
            ctx = await browser.new_context(
                user_agent=(
                    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
                    "AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36"
                ),
                viewport={"width": 1440, "height": 900},
                locale="zh-CN",
            )
            page = await ctx.new_page()
            await Stealth().apply_stealth_async(page)

            # Step 1: Resolve Aliyun WAF to get xq_a_token
            if not await resolve_waf(page, ctx):
                emit({"type": "error", "message": "WAF resolution failed"})
                await browser.close()
                return

            # Step 2: Navigate to a profile page — triggers login modal
            await page.goto(
                "https://xueqiu.com/u/1247347556",
                wait_until="domcontentloaded",
                timeout=60000,
            )
            await page.wait_for_timeout(5000)

            # Step 3: Click the "二维码登录" tab
            clicked = await page.evaluate("""() => {
                const els = Array.from(document.querySelectorAll('a, span, div, button'));
                for (const el of els) {
                    if (el.textContent.trim() === '二维码登录') {
                        el.click();
                        return true;
                    }
                }
                return false;
            }""")
            if not clicked:
                emit({"type": "error", "message": "QR tab not found"})
                await browser.close()
                return

            await page.wait_for_timeout(3000)

            # Step 4: Wait for QR code canvas to render and capture it.
            # Xueqiu renders the login QR as a <canvas> inside .qrcode__wrapper
            qr_b64 = None
            for _ in range(15):
                qr_b64 = await page.evaluate("""() => {
                    const wrapper = document.querySelector('.qrcode__wrapper');
                    if (!wrapper) return null;
                    const canvas = wrapper.querySelector('canvas');
                    if (!canvas || canvas.width < 50) return null;
                    try {
                        const dataUrl = canvas.toDataURL('image/png');
                        const b64 = dataUrl.split(',')[1];
                        return (b64 && b64.length > 100) ? b64 : null;
                    } catch { return null; }
                }""")
                if qr_b64:
                    break
                await page.wait_for_timeout(1000)

            # Fallback: screenshot the wrapper element
            if not qr_b64:
                wrapper = await page.query_selector(".qrcode__wrapper")
                if wrapper and await wrapper.is_visible():
                    shot = await wrapper.screenshot(timeout=10000)
                    qr_b64 = base64.b64encode(shot).decode()

            if not qr_b64:
                emit({"type": "error", "message": "Failed to capture QR code"})
                await browser.close()
                return

            emit({"type": "qr", "image": qr_b64})

            # Step 6: Poll for login success
            start = time.time()
            while time.time() - start < timeout:
                cookies = await ctx.cookies()
                cookie_dict = {c["name"]: c["value"] for c in cookies}

                # Only xq_is_login is reliable; the "u" cookie is set by WAF
                # even without login.
                is_logged_in = cookie_dict.get("xq_is_login") == "1"

                if is_logged_in:
                    # Fetch user info
                    user_info = await page.evaluate("""async () => {
                        try {
                            const resp = await fetch(
                                '/v4/user/info/me.json',
                                {credentials: 'include',
                                 headers: {'Accept': 'application/json'}},
                            );
                            const data = await resp.json();
                            return data;
                        } catch { return null; }
                    }""")

                    # Build cookie string
                    cookie_str = "; ".join(
                        f"{c['name']}={c['value']}"
                        for c in cookies
                        if c["domain"].endswith("xueqiu.com")
                    )

                    nickname = ""
                    uid = ""
                    avatar = ""
                    if user_info and not user_info.get("error_code"):
                        nickname = user_info.get("screen_name", "")
                        uid = str(user_info.get("id", ""))
                        avatar = user_info.get("profile_image_url", "")
                    elif cookie_dict.get("u"):
                        uid = cookie_dict["u"]

                    emit({
                        "type": "success",
                        "cookies": cookie_str,
                        "nickname": nickname,
                        "uid": uid,
                        "avatar": avatar,
                    })
                    await browser.close()
                    return

                # Check if QR code expired
                expired = await page.evaluate("""() => {
                    const text = document.body.textContent || '';
                    return text.includes('已过期') || text.includes('刷新二维码');
                }""")
                if expired:
                    emit({"type": "expired"})
                    await browser.close()
                    return

                await page.wait_for_timeout(2000)

            emit({"type": "expired"})
            await browser.close()

    except Exception as e:
        emit({"type": "error", "message": str(e)})


if __name__ == "__main__":
    asyncio.run(main())
