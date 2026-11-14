import {mkdtempSync, readFileSync, rmSync} from 'node:fs'
import {tmpdir} from 'node:os'
import {join} from 'node:path'
import {spawnSync} from 'node:child_process'
import {fileURLToPath} from 'node:url'

const app = fileURLToPath(new URL('../../apps/operator-ui/', import.meta.url))
const cli = join(app, 'node_modules/openapi-typescript/bin/cli.js')
const saved = join(app, 'src/lib/openapi.generated.ts')
const folder = mkdtempSync(join(tmpdir(), 'emad-contracts-'))
try {
  const generated = join(folder, 'openapi.generated.ts')
  const result = spawnSync(process.execPath, [cli, '../../api/openapi/openapi.yaml', '-o', generated], {cwd: app, stdio: 'inherit'})
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error('OpenAPI generation failed')
  const canonical = (path) => readFileSync(path, 'utf8').replaceAll('\r\n', '\n')
  if (canonical(saved) !== canonical(generated)) throw new Error('Generated client differs. Run pnpm --dir apps/operator-ui contracts:generate.')
  console.log('Generated client matches the OpenAPI source.')
} finally {
  rmSync(folder, {recursive: true, force: true})
}
