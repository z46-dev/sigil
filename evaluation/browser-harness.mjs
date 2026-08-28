import { createServer } from "node:http";
import { mkdir, readFile, stat, writeFile } from "node:fs/promises";
import { extname, join, normalize, resolve } from "node:path";
import { chromium, firefox, webkit } from "playwright";

const publicDirectory = resolve("client/public");
const outputPath = resolve("evaluation/reports/observations.json");
const mode = process.env.SIGIL_EVALUATION_MODE || "device";
const repeats = Number.parseInt(process.env.SIGIL_EVALUATION_REPEATS || "2", 10);
const requestedBrowsers = (process.env.SIGIL_EVALUATION_BROWSERS || "chromium,firefox,webkit").split(",");
const browserTypes = { chromium, firefox, webkit };
const contentTypes = {
    ".html": "text/html; charset=utf-8",
    ".js": "text/javascript; charset=utf-8",
    ".wasm": "application/wasm",
};

const startServer = async () => {
    await stat(join(publicDirectory, "main.wasm"));

    const server = createServer(async (request, response) => {
        try {
            const requestPath = request.url === "/" ? "/index.html" : request.url.split("?", 1)[0];
            const filePath = normalize(join(publicDirectory, requestPath));
            if (!filePath.startsWith(publicDirectory)) {
                response.writeHead(403).end();
                return;
            }

            const contents = await readFile(filePath);
            response.writeHead(200, { "Content-Type": contentTypes[extname(filePath)] || "application/octet-stream" });
            response.end(contents);
        } catch {
            response.writeHead(404).end();
        }
    });

    await new Promise(resolveListening => server.listen(0, "127.0.0.1", resolveListening));
    const address = server.address();
    return {
        server: server,
        url: `http://127.0.0.1:${address.port}`
    };
};

const collect = async (context, url, browser, scenario, label) => {
    const page = await context.newPage();
    try {
        await page.goto(url);
        await page.waitForFunction(() => Boolean(window.sigil?.collect));
        return {
            label,
            browser,
            scenario,
            snapshot: await page.evaluate(selectedMode => window.sigil.collect({ mode: selectedMode }), mode),
        };
    } finally {
        await page.close();
    }
};

const collectBrowser = async (name, browserType, url) => {
    const browser = await browserType.launch({ headless: true });
    const observations = [];
    try {
        const session = await browser.newContext();
        try {
            for (let repeat = 0; repeat < repeats; repeat += 1) {
                observations.push(await collect(session, url, name, `session-${repeat + 1}`, "physical-host"));
            }
        } finally {
            await session.close();
        }

        const isolated = await browser.newContext();
        try {
            observations.push(await collect(isolated, url, name, "isolated-context", "physical-host"));
        } finally {
            await isolated.close();
        }

        const mobile = await browser.newContext({
            viewport: {
                width: 412,
                height: 915
            },
            screen: {
                width: 412,
                height: 915
            },
            deviceScaleFactor: 2.625,
            hasTouch: true,
            userAgent: "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Mobile Safari/537.36",
        });
        try {
            observations.push(await collect(mobile, url, name, "emulated-mobile", "synthetic-mobile"));
        } finally {
            await mobile.close();
        }
    } finally {
        await browser.close();
    }

    return observations;
};

const { server, url } = await startServer();
const observations = [];

try {
    for (const name of requestedBrowsers) {
        const browserType = browserTypes[name];
        if (!browserType) {
            throw new Error(`Unsupported browser: ${name}`);
        }
        observations.push(...await collectBrowser(name, browserType, url));
    }
} finally {
    await new Promise((resolveClosed, rejectClosed) => server.close((error) => error ? rejectClosed(error) : resolveClosed()));
}

await mkdir(resolve("evaluation/reports"), { recursive: true });
await writeFile(outputPath, `${JSON.stringify(observations, null, 2)}\n`);
console.log(`Collected ${observations.length} observations in ${outputPath}`);
