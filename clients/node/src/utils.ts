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

/**
 * Returns n free TCP ports allocated atomically: all sockets are held open
 * simultaneously so the OS cannot reuse them for each other, then released
 * together.  This avoids the race condition that arises from calling
 * getFreePort() sequentially.
 */
export function getFreePorts(n: number): Promise<number[]> {
  return new Promise((resolve, reject) => {
    const servers: net.Server[] = []
    const ports: number[] = []
    let opened = 0

    for (let i = 0; i < n; i++) {
      const srv = net.createServer()
      servers.push(srv)
      srv.listen(0, '127.0.0.1', () => {
        ports.push((srv.address() as net.AddressInfo).port)
        if (++opened === n) {
          let closed = 0
          for (const s of servers) {
            s.close((err) => {
              if (err) { reject(err); return }
              if (++closed === n) resolve(ports)
            })
          }
        }
      })
    }
  })
}

export function sleep(ms: number): Promise<void> {
  return new Promise((r) => setTimeout(r, ms))
}
