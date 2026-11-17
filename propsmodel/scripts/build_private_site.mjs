import { mkdtempSync, cpSync, existsSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, resolve, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { execFileSync } from 'node:child_process'
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..')
const site=join(root,'site')
const safePublic=mkdtempSync(join(tmpdir(),'wraith-public-'))
try {
 for (const name of ['brand','manifest.webmanifest']) {
  const source=join(site,'public',name)
  if (existsSync(source)) cpSync(source,join(safePublic,name),{recursive:true})
 }
 execFileSync(process.execPath,[join(site,'node_modules/vite/bin/vite.js'),'build'],{
  cwd:site,stdio:'inherit',env:{...process.env,VITE_PRIVATE_BOARD:'true',WRAITH_PRIVATE_PUBLIC_DIR:safePublic},
 })
 if (existsSync(join(site,'dist','data'))) throw new Error('Private build must not contain static pick data.')
} finally { rmSync(safePublic,{recursive:true,force:true}) }
