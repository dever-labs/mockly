import http from 'http'
import https from 'https'
import tls from 'tls'
import { createWriteStream, existsSync, mkdirSync, chmodSync } from 'fs'
import { join, resolve } from 'path'

/** Version of the Mockly binary this package was tested against. */
export const DEFAULT_MOCKLY_VERSION = 'v0.1.0'

/** Default GitHub releases base URL. */
const GITHUB_BASE = 'https://github.com/dever-labs/mockly/releases/download'

export interface InstallOptions {
  /**
   * Mockly version to install.
   * @default process.env.MOCKLY_VERSION ?? DEFAULT_MOCKLY_VERSION
   */
  version?: string

  /**
   * Base URL for downloading release assets.
   *
   * Override this to route downloads through Artifactory or an internal mirror:
   * ```
   * MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.company.com/artifactory/github-releases/dever-labs/mockly/releases/download
   * ```
   * The full download URL becomes `${baseUrl}/${version}/${assetName}`.
   *
   * @default process.env.MOCKLY_DOWNLOAD_BASE_URL ?? GitHub releases URL
   */
  baseUrl?: string

  /**
   * Directory to place the downloaded binary.
   * @default path.join(process.cwd(), 'bin')
   */
  binDir?: string

  /**
   * Re-download even if the binary already exists.
   * @default false
   */
  force?: boolean
}

/**
 * Returns the path to the installed binary, or `null` if not found.
 *
 * Resolution order:
 * 1. `MOCKLY_BINARY_PATH` env var (absolute path to pre-staged binary)
 * 2. `<binDir>/mockly[.exe]` (downloaded by `install()`)
 * 3. `<cwd>/node_modules/.bin/mockly[.exe]` (future: native npm wrapper)
 */
export function getBinaryPath(binDir?: string): string | null {
  const ext = process.platform === 'win32' ? '.exe' : ''

  if (process.env.MOCKLY_BINARY_PATH) {
    const p = resolve(process.env.MOCKLY_BINARY_PATH)
    if (existsSync(p)) return p
  }

  const dir = binDir ?? join(process.cwd(), 'bin')
  const fromBinDir = join(dir, `mockly${ext}`)
  if (existsSync(fromBinDir)) return fromBinDir

  const fromModules = resolve(process.cwd(), 'node_modules', '.bin', `mockly${ext}`)
  if (existsSync(fromModules)) return fromModules

  return null
}

/**
 * Downloads (or locates) the Mockly binary and returns its path.
 *
 * Environment variables (all optional):
 * - `MOCKLY_BINARY_PATH`       — use this exact binary; skips all download logic
 * - `MOCKLY_NO_INSTALL`        — throw instead of downloading (for air-gapped envs
 *                                 where the binary is staged externally)
 * - `MOCKLY_DOWNLOAD_BASE_URL` — base URL override for Artifactory / internal mirrors
 * - `MOCKLY_VERSION`           — binary version override
 * - `HTTPS_PROXY` / `HTTP_PROXY` — route the download through an HTTP proxy
 *
 * @returns Absolute path to the installed binary.
 */
export async function install(opts: InstallOptions = {}): Promise<string> {
  const ext = process.platform === 'win32' ? '.exe' : ''
  const binDir = opts.binDir ?? join(process.cwd(), 'bin')
  const binPath = join(binDir, `mockly${ext}`)

  // 1. MOCKLY_BINARY_PATH — use a pre-staged binary without downloading
  if (process.env.MOCKLY_BINARY_PATH) {
    const staged = resolve(process.env.MOCKLY_BINARY_PATH)
    if (!existsSync(staged)) {
      throw new Error(
        `MOCKLY_BINARY_PATH is set to "${staged}" but the file does not exist.\n` +
        `Stage the binary for platform "${process.platform}/${process.arch}" before running tests.`
      )
    }
    return staged
  }

  // 2. Already installed — skip unless force
  if (!opts.force && existsSync(binPath)) {
    return binPath
  }

  // 3. MOCKLY_NO_INSTALL — fail fast with actionable instructions
  if (process.env.MOCKLY_NO_INSTALL) {
    throw new Error(
      `MOCKLY_NO_INSTALL is set but no Mockly binary was found.\n\n` +
      `To resolve this, choose one of:\n` +
      `  a) Stage the binary manually:\n` +
      `       Place the binary at: ${binPath}\n` +
      `       Download from: https://github.com/dever-labs/mockly/releases\n\n` +
      `  b) Set MOCKLY_BINARY_PATH to the absolute path of an existing binary.\n\n` +
      `  c) Use MOCKLY_DOWNLOAD_BASE_URL to point to an internal mirror:\n` +
      `       MOCKLY_DOWNLOAD_BASE_URL=https://artifactory.company.com/artifactory/github-releases/dever-labs/mockly/releases/download`
    )
  }

  // 4. Download
  const version = opts.version ?? process.env.MOCKLY_VERSION ?? DEFAULT_MOCKLY_VERSION
  const baseUrl = opts.baseUrl ?? process.env.MOCKLY_DOWNLOAD_BASE_URL ?? GITHUB_BASE
  const asset = getAssetName()
  const url = `${baseUrl}/${version}/${asset}`

  mkdirSync(binDir, { recursive: true })

  console.log(`mockly-node: downloading ${asset} from ${url}`)
  await downloadFile(url, binPath)

  if (process.platform !== 'win32') {
    chmodSync(binPath, 0o755)
  }

  console.log(`mockly-node: installed at ${binPath}`)
  return binPath
}

// ─── Asset name ───────────────────────────────────────────────────────────────

const ARCH_MAP: Record<string, string> = {
  x64: 'amd64',
  arm64: 'arm64',
}

function getAssetName(): string {
  const os = process.platform === 'win32' ? 'windows'
    : process.platform === 'darwin' ? 'darwin'
    : 'linux'

  const arch = ARCH_MAP[process.arch]
  if (!arch) {
    throw new Error(
      `Unsupported architecture: ${process.arch}.\n` +
      `Supported: x64 (amd64), arm64.\n` +
      `For other platforms, build from source: https://github.com/dever-labs/mockly`
    )
  }

  return `mockly-${os}-${arch}${process.platform === 'win32' ? '.exe' : ''}`
}

// ─── Download ─────────────────────────────────────────────────────────────────

function getProxyUrl(): string | undefined {
  return (
    process.env.HTTPS_PROXY ??
    process.env.https_proxy ??
    process.env.npm_config_https_proxy ??
    process.env.HTTP_PROXY ??
    process.env.http_proxy ??
    process.env.npm_config_proxy
  )
}

function downloadFile(url: string, dest: string): Promise<void> {
  const proxyUrl = getProxyUrl()
  return proxyUrl ? downloadViaProxy(url, dest, proxyUrl) : downloadDirect(url, dest)
}

function downloadDirect(url: string, dest: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const get = (u: string) => {
      https.get(u, { headers: { 'User-Agent': 'mockly-node-install' } }, (res) => {
        if (res.statusCode === 301 || res.statusCode === 302) {
          get(res.headers.location!)
          return
        }
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP ${res.statusCode} from ${u}`))
          return
        }
        const ws = createWriteStream(dest)
        res.pipe(ws)
        ws.on('finish', resolve)
        ws.on('error', reject)
      }).on('error', reject)
    }
    get(url)
  })
}

/**
 * Downloads via HTTP CONNECT proxy (for environments where HTTPS_PROXY /
 * HTTP_PROXY is set but MOCKLY_DOWNLOAD_BASE_URL cannot be used).
 *
 * For Artifactory, prefer setting MOCKLY_DOWNLOAD_BASE_URL instead — it is
 * simpler and avoids CONNECT tunnel complexity.
 */
function downloadViaProxy(targetUrl: string, dest: string, proxyUrl: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const target = new URL(targetUrl)
    const proxy = new URL(proxyUrl)

    const connectOpts: http.RequestOptions = {
      host: proxy.hostname,
      port: parseInt(proxy.port) || 3128,
      method: 'CONNECT',
      path: `${target.hostname}:443`,
      headers: {},
    }

    if (proxy.username) {
      const auth = Buffer.from(`${decodeURIComponent(proxy.username)}:${decodeURIComponent(proxy.password)}`).toString('base64')
      ;(connectOpts.headers as Record<string, string>)['Proxy-Authorization'] = `Basic ${auth}`
    }

    const connectReq = http.request(connectOpts)

    connectReq.on('connect', (_res, socket) => {
      // Wrap the plain socket in TLS to complete the HTTPS tunnel
      const tlsSocket = tls.connect({ socket, servername: target.hostname })

      const req = https.request(
        {
          createConnection: () => tlsSocket,
          hostname: target.hostname,
          path: target.pathname + target.search,
          method: 'GET',
          headers: { 'User-Agent': 'mockly-node-install' },
        },
        (res) => {
          if (res.statusCode === 301 || res.statusCode === 302) {
            tlsSocket.destroy()
            downloadViaProxy(res.headers.location!, dest, proxyUrl).then(resolve).catch(reject)
            return
          }
          if (res.statusCode !== 200) {
            reject(new Error(`HTTP ${res.statusCode} from ${targetUrl} (via proxy ${proxyUrl})`))
            return
          }
          const ws = createWriteStream(dest)
          res.pipe(ws)
          ws.on('finish', resolve)
          ws.on('error', reject)
        }
      )
      req.on('error', reject)
      req.end()
    })

    connectReq.on('error', reject)
    connectReq.end()
  })
}
