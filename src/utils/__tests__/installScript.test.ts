import { chmodSync, existsSync, mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { spawnSync } from 'node:child_process'
import { afterEach, describe, expect, it } from 'vitest'

function findPackageRoot(): string {
  let dir = import.meta.dirname
  for (let i = 0; i < 10; i++) {
    if (existsSync(join(dir, 'package.json')))
      return dir
    dir = join(dir, '..')
  }
  throw new Error('Could not find package root')
}

const packageRoot = findPackageRoot()
const installScript = join(packageRoot, 'install.sh')
const tempDirs: string[] = []

function createTempDir(): string {
  const dir = mkdtempSync(join(tmpdir(), 'ccg-install-test-'))
  tempDirs.push(dir)
  return dir
}

function findExecutable(name: string): string {
  const result = spawnSync('which', [name], { encoding: 'utf8' })
  if (result.status !== 0)
    throw new Error(`Could not find executable: ${name}`)
  return result.stdout.trim()
}

afterEach(() => {
  for (const dir of tempDirs.splice(0))
    rmSync(dir, { force: true, recursive: true })
})

describe('install.sh', () => {
  it('builds and installs codeagent-wrapper under ~/.claude/bin', () => {
    const tempDir = createTempDir()
    const home = join(tempDir, 'home')

    const result = spawnSync('bash', [installScript], {
      cwd: tempDir,
      encoding: 'utf8',
      env: { ...process.env, HOME: home },
    })

    expect(result.status, result.stderr).toBe(0)

    const installedBinary = join(home, '.claude', 'bin', 'codeagent-wrapper')
    expect(existsSync(installedBinary)).toBe(true)

    const versionResult = spawnSync(installedBinary, ['--version'], { encoding: 'utf8' })
    expect(versionResult.status, versionResult.stderr).toBe(0)
    expect(versionResult.stdout).toMatch(/^codeagent-wrapper version \S+/)
  }, 20_000)

  it('does not overwrite an existing installation when compilation fails', () => {
    const tempDir = createTempDir()
    const home = join(tempDir, 'home')
    const installDir = join(home, '.claude', 'bin')
    const installedBinary = join(installDir, 'codeagent-wrapper')
    mkdirSync(installDir, { recursive: true })
    writeFileSync(installedBinary, 'existing binary')

    const fakeBin = join(tempDir, 'bin')
    const fakeGo = join(fakeBin, 'go')
    mkdirSync(fakeBin)
    writeFileSync(fakeGo, '#!/usr/bin/env bash\nexit 42\n')
    chmodSync(fakeGo, 0o755)

    const result = spawnSync('bash', [installScript], {
      cwd: tempDir,
      encoding: 'utf8',
      env: { ...process.env, HOME: home, PATH: `${fakeBin}:${process.env.PATH}` },
    })

    expect(result.status).toBe(42)
    expect(readFileSync(installedBinary, 'utf8')).toBe('existing binary')
  })

  it('rejects an installation path that is already a directory', () => {
    const tempDir = createTempDir()
    const home = join(tempDir, 'home')
    const installedBinary = join(home, '.claude', 'bin', 'codeagent-wrapper')
    mkdirSync(installedBinary, { recursive: true })

    const result = spawnSync('bash', [installScript], {
      cwd: tempDir,
      encoding: 'utf8',
      env: { ...process.env, HOME: home },
    })

    expect(result.status).not.toBe(0)
    expect(result.stderr).toContain('is a directory')
    expect(existsSync(join(installedBinary, 'codeagent-wrapper'))).toBe(false)
  })

  it('rejects a directory created at the installation path during compilation', () => {
    const tempDir = createTempDir()
    const home = join(tempDir, 'home')
    const installedBinary = join(home, '.claude', 'bin', 'codeagent-wrapper')
    const fakeBin = join(tempDir, 'bin')
    const fakeGo = join(fakeBin, 'go')
    const realGo = findExecutable('go')
    mkdirSync(fakeBin)
    writeFileSync(
      fakeGo,
      `#!/usr/bin/env bash\n"${realGo}" "$@" || exit $?\nif [[ "\${1:-}" == "build" ]]; then\n  mkdir -p "\${HOME}/.claude/bin/codeagent-wrapper"\nfi\n`,
    )
    chmodSync(fakeGo, 0o755)

    const result = spawnSync('bash', [installScript], {
      cwd: tempDir,
      encoding: 'utf8',
      env: { ...process.env, HOME: home, PATH: `${fakeBin}:${process.env.PATH}` },
    })

    expect(result.status).not.toBe(0)
    expect(existsSync(join(installedBinary, 'codeagent-wrapper'))).toBe(false)
  }, 20_000)

  it('preserves an existing installation when the atomic replacement fails', () => {
    const tempDir = createTempDir()
    const home = join(tempDir, 'home')
    const installDir = join(home, '.claude', 'bin')
    const installedBinary = join(installDir, 'codeagent-wrapper')
    mkdirSync(installDir, { recursive: true })
    writeFileSync(installedBinary, 'existing binary')

    const fakeBin = join(tempDir, 'bin')
    const fakeGo = join(fakeBin, 'go')
    const realGo = findExecutable('go')
    mkdirSync(fakeBin)
    writeFileSync(
      fakeGo,
      `#!/usr/bin/env bash\nif [[ "\${1:-}" == "run" ]]; then\n  exit 43\nfi\nexec "${realGo}" "$@"\n`,
    )
    chmodSync(fakeGo, 0o755)

    const result = spawnSync('bash', [installScript], {
      cwd: tempDir,
      encoding: 'utf8',
      env: { ...process.env, HOME: home, PATH: `${fakeBin}:${process.env.PATH}` },
    })

    expect(result.status).toBe(43)
    expect(readFileSync(installedBinary, 'utf8')).toBe('existing binary')
  }, 20_000)
})
