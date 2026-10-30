"use strict";

// Tests for install.js's downloads: proxy selection, the CONNECT tunnel,
// redirects and the time limit. Run with: node --test npm/

const assert = require("assert");
const http = require("http");
const net = require("net");
const { after, before, describe, it } = require("node:test");

const { download, proxyForUrl } = require("./install.js");

describe("proxyForUrl", () => {
  const proxy = (url, env) => {
    const result = proxyForUrl(url, env);
    return result && result.href;
  };

  it("uses HTTPS_PROXY for https URLs", () => {
    assert.strictEqual(proxy("https://github.com/x", { HTTPS_PROXY: "http://proxy:3128" }), "http://proxy:3128/");
  });

  it("accepts lower case and prefers upper case", () => {
    assert.strictEqual(proxy("https://github.com/x", { https_proxy: "http://lower:1" }), "http://lower:1/");
    assert.strictEqual(
      proxy("https://github.com/x", { HTTPS_PROXY: "http://upper:1", https_proxy: "http://lower:1" }),
      "http://upper:1/"
    );
  });

  it("uses HTTP_PROXY only for http URLs", () => {
    assert.strictEqual(proxy("https://github.com/x", { HTTP_PROXY: "http://proxy:3128" }), null);
    assert.strictEqual(proxy("http://github.com/x", { HTTP_PROXY: "http://proxy:3128" }), "http://proxy:3128/");
  });

  it("adds http:// to a proxy without a scheme", () => {
    assert.strictEqual(proxy("https://github.com/x", { HTTPS_PROXY: "proxy:3128" }), "http://proxy:3128/");
  });

  it("rejects proxy schemes it can't use", () => {
    assert.throws(() => proxyForUrl("https://github.com/x", { HTTPS_PROXY: "socks5://proxy:1080" }), /socks5/);
  });

  it("never proxies localhost or loopback addresses", () => {
    const env = { HTTPS_PROXY: "http://proxy:3128" };
    for (const url of ["https://localhost/x", "https://127.0.0.1/x", "https://127.1.2.3/x", "https://[::1]/x"]) {
      assert.strictEqual(proxy(url, env), null, url);
    }
  });

  it("follows Go's NO_PROXY rules", () => {
    const cases = [
      ["*", "https://github.com/x", true],
      ["github.com", "https://github.com/x", true],
      ["github.com", "https://api.github.com/x", true],
      ["github.com", "https://notgithub.com/x", false],
      [".github.com", "https://github.com/x", false],
      [".github.com", "https://api.github.com/x", true],
      ["*.github.com", "https://api.github.com/x", true],
      ["GitHub.COM", "https://github.com/x", true],
      ["github.com:443", "https://github.com/x", true],
      ["github.com:8443", "https://github.com/x", false],
      ["10.0.0.0/8", "https://10.1.2.3/x", true],
      ["10.0.0.0/8", "https://11.1.2.3/x", false],
      ["10.1.2.3", "https://10.1.2.3/x", true],
      ["10.0.0.0/abc", "https://10.1.2.3/x", false],
      ["10.0.0.0/99", "https://10.1.2.3/x", false],
      ["10.0.0.0/99, 10.1.2.3", "https://10.1.2.3/x", true],
      ["other.com, github.com", "https://github.com/x", true],
      ["", "https://github.com/x", false]
    ];
    for (const [noProxy, url, bypass] of cases) {
      const result = proxy(url, { HTTPS_PROXY: "http://proxy:3128", NO_PROXY: noProxy });
      assert.strictEqual(result === null, bypass, `NO_PROXY=${JSON.stringify(noProxy)} ${url}`);
    }
  });

  it("accepts lower case no_proxy", () => {
    assert.strictEqual(proxy("https://github.com/x", { HTTPS_PROXY: "http://proxy:3128", no_proxy: "github.com" }), null);
  });
});

describe("download", () => {
  let origin;
  let originPort;
  let tunnelProxy;
  let tunnelProxyPort;
  const connects = [];

  before(async () => {
    origin = http.createServer((req, res) => {
      if (req.url === "/host") {
        res.end(req.headers.host);
      } else if (req.url === "/file") {
        res.end("archive bytes");
      } else if (req.url === "/redirect") {
        res.writeHead(302, { location: "/file" });
        res.end();
      } else if (req.url === "/loop") {
        res.writeHead(302, { location: "/loop" });
        res.end();
      } else if (req.url === "/stall") {
        res.writeHead(200);
        res.write("partial");
      } else {
        res.writeHead(404);
        res.end();
      }
    });
    originPort = await listen(origin);

    // A CONNECT proxy that sends every tunnel to the local origin, so tests can
    // use a host name that is not a loopback address.
    tunnelProxy = http.createServer();
    tunnelProxy.on("connect", (req, clientSocket) => {
      connects.push({ authority: req.url, authorization: req.headers["proxy-authorization"] });
      if (req.url.startsWith("refused.test")) {
        clientSocket.end("HTTP/1.1 403 Forbidden\r\n\r\n");
        return;
      }
      const upstream = net.connect(originPort, "127.0.0.1", () => {
        clientSocket.write("HTTP/1.1 200 Connection Established\r\n\r\n");
        upstream.pipe(clientSocket);
        clientSocket.pipe(upstream);
      });
      upstream.on("error", () => clientSocket.destroy());
      clientSocket.on("error", () => upstream.destroy());
    });
    tunnelProxyPort = await listen(tunnelProxy);
  });

  after(() => {
    origin.closeAllConnections();
    origin.close();
    tunnelProxy.close();
  });

  it("downloads directly without a proxy", async () => {
    const data = await download(`http://127.0.0.1:${originPort}/file`, { env: {} });
    assert.strictEqual(data.toString(), "archive bytes");
  });

  it("downloads through the proxy's CONNECT tunnel", async () => {
    const env = { HTTP_PROXY: `http://user:p%40ss@127.0.0.1:${tunnelProxyPort}` };
    const data = await download(`http://download.test:${originPort}/file`, { env });
    assert.strictEqual(data.toString(), "archive bytes");
    const connect = connects.at(-1);
    assert.strictEqual(connect.authority, `download.test:${originPort}`);
    assert.strictEqual(connect.authorization, `Basic ${Buffer.from("user:p@ss").toString("base64")}`);
  });

  it("sends the target's Host header through the tunnel", async () => {
    const env = { HTTP_PROXY: `http://127.0.0.1:${tunnelProxyPort}` };
    const data = await download(`http://download.test:${originPort}/host`, { env });
    assert.strictEqual(data.toString(), `download.test:${originPort}`);
  });

  it("follows redirects through the proxy", async () => {
    const env = { HTTP_PROXY: `http://127.0.0.1:${tunnelProxyPort}` };
    const data = await download(`http://download.test:${originPort}/redirect`, { env });
    assert.strictEqual(data.toString(), "archive bytes");
  });

  it("stops after too many redirects", async () => {
    await assert.rejects(download(`http://127.0.0.1:${originPort}/loop`, { env: {} }), /too many redirects/);
  });

  it("names the URL, not the proxy credentials, when the proxy refuses", async () => {
    const env = { HTTP_PROXY: `http://user:secret@127.0.0.1:${tunnelProxyPort}` };
    const url = `http://refused.test:${originPort}/file`;
    await assert.rejects(download(url, { env }), (error) => {
      assert.match(error.message, /refused to connect to refused\.test/);
      assert.ok(error.message.includes(url), error.message);
      assert.ok(!error.message.includes("secret"), error.message);
      return true;
    });
  });

  it("fails when the proxy can't be reached", async () => {
    const closedPort = await unusedPort();
    const env = { HTTP_PROXY: `http://127.0.0.1:${closedPort}` };
    await assert.rejects(download(`http://download.test:${originPort}/file`, { env }), /ECONNREFUSED/);
  });

  it("fails when no data arrives within the time limit", async () => {
    const url = `http://127.0.0.1:${originPort}/stall`;
    const started = Date.now();
    await assert.rejects(download(url, { env: {}, timeoutMs: 200 }), (error) => {
      assert.ok(error.message.includes(url), error.message);
      assert.match(error.message, /no data for 0\.2s/);
      return true;
    });
    assert.ok(Date.now() - started < 5000);
  });

  it("reports the status for a missing file", async () => {
    await assert.rejects(download(`http://127.0.0.1:${originPort}/missing`, { env: {} }), /\(404\)/);
  });
});

function listen(server) {
  return new Promise((resolve) => server.listen(0, "127.0.0.1", () => resolve(server.address().port)));
}

async function unusedPort() {
  const server = net.createServer();
  const port = await listen(server);
  await new Promise((resolve) => server.close(resolve));
  return port;
}
