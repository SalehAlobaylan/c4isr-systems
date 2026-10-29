// Stable loopback entry for the regular development UI. The UI's own proxy
// owns API routing; this process neither launches an evaluation API nor expires.
import http from 'node:http'
import net from 'node:net'

const args = process.argv.slice(2)
const option = (name, fallback) => {
  const index = args.indexOf(name)
  return index < 0 ? fallback : args[index + 1]
}
const port = Number(option('--port', '5198'))
const target = new URL(option('--target', 'http://localhost:5173'))
if (!Number.isInteger(port) || port < 1024 || port > 65535 || target.protocol !== 'http:' ||
    !['localhost', '127.0.0.1', '[::1]'].includes(target.hostname) || target.username || target.password ||
    target.pathname !== '/' || target.search || target.hash) {
  throw new Error('Use a loopback HTTP target and a valid local port.')
}
const hostname = target.hostname.replace(/^\[|\]$/g, '')
const targetPort = Number(target.port || 80)
const server = http.createServer((request, response) => {
  if (request.url?.split('?')[0] === '/__local/open') {
    response.writeHead(200, {'Content-Type':'text/html; charset=utf-8', 'Cache-Control':'no-store'})
    response.end(`<!doctype html><title>Opening EMAD</title><script>sessionStorage.removeItem('emad.session-token');location.replace('/map')</script>`)
    return
  }
  const upstream = http.request({hostname, port:targetPort, path:request.url, method:request.method,
    headers:request.headers, timeout:60_000}, reply => {
    response.writeHead(reply.statusCode ?? 502, {...reply.headers, 'cache-control':'no-store'})
    reply.pipe(response)
  })
  upstream.on('timeout', () => upstream.destroy(new Error('Local UI timeout')))
  upstream.on('error', () => {
    if (!response.headersSent) {
      response.writeHead(502, {'Content-Type':'text/html; charset=utf-8', 'Cache-Control':'no-store'})
      response.end('<!doctype html><title>EMAD connection unavailable</title><h1>EMAD server is not running</h1><p>Start the regular local API and UI with task dev, then reload this page. An account credential is not required.</p>')
    } else response.destroy()
  })
  request.on('aborted', () => upstream.destroy())
  request.pipe(upstream)
})
server.on('upgrade', (request, socket, head) => {
  const upstream = net.connect({host:hostname, port:targetPort}, () => {
    upstream.write(`${request.method} ${request.url} HTTP/1.1\r\n` +
      Object.entries(request.headers).map(([key, value]) => `${key}: ${value}`).join('\r\n') + '\r\n\r\n')
    if (head.length) upstream.write(head)
    socket.pipe(upstream).pipe(socket)
  })
  upstream.on('error', () => socket.destroy())
  socket.on('error', () => upstream.destroy())
  socket.on('close', () => upstream.destroy())
})
server.listen(port, '127.0.0.1', () => console.log(`EMAD local entry: http://127.0.0.1:${port}/map → ${target.origin}`))
