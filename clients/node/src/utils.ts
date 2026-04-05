import net from 'net'

/**
 * Returns a free TCP port on 127.0.0.1.
 * Always allocate ports sequentially, never in parallel, to avoid TOCTOU
 * races where two concurrent calls could receive the same port.
 */
export function getFreePort(): Promise<number> {
  return new Promise((resolve) => {
    const srv = net.createServer()
    srv.listen(0, '127.0.0.1', () => {
      const port = (srv.address() as net.AddressInfo).port
      srv.close(() => resolve(port))
    })
  })
}

export function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms))
}
