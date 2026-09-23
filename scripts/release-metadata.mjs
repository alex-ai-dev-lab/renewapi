import fs from 'node:fs'
import path from 'node:path'
import { pathToFileURL } from 'node:url'

export function releaseMetadata(input) {
  if (!/^[a-f0-9]{40}$/.test(input.sha) || /\s/.test(input.sha)) throw new Error('源码 SHA 无效')
  if (input.expectedSha && input.expectedSha !== input.sha) {
    throw new Error('实际源码与授权发布 SHA 不一致')
  }
  if (!/^[1-9][0-9]*$/.test(input.runId) || /\s/.test(input.runId)) throw new Error('运行编号无效')
  if (!/^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(input.version) || /\s/.test(input.version)) {
    throw new Error('VERSION 必须是独立产品版本')
  }

  const short = input.sha.slice(0, 12)
  const product = input.ref.startsWith('refs/tags/renewapi-v')
  let tag = input.imageTag || `sha-${short}`
  let releaseTag = input.releaseTag || `renewapi-build-${short}-${input.runId}`
  let channel = input.ref === 'refs/heads/main' ? 'edge' : 'manual'
  let prerelease = true
  let name = `RenewAPI ${input.version} 构建 ${short}`

  if (product) {
    releaseTag = input.ref.slice('refs/tags/'.length)
    if (releaseTag !== `renewapi-${input.version}`) {
      throw new Error('产品标签与 VERSION 不一致')
    }
    if (input.imageTag || input.releaseTag) {
      throw new Error('产品标签发布不接受源码包身份覆盖')
    }
    tag = input.version.slice(1)
    channel = 'release'
    prerelease = input.version.includes('-')
    name = `RenewAPI ${input.version}`
  } else {
    if (!/^renewapi-(?:build|source)-[A-Za-z0-9][A-Za-z0-9._-]*$/.test(releaseTag) || /\s/.test(releaseTag)) {
      throw new Error('源码包标签必须使用 renewapi-build- 或 renewapi-source- 前缀')
    }
    if (['latest', 'edge', 'rc'].includes(tag)) {
      throw new Error('源码镜像必须使用可追溯标签')
    }
  }
  if (!/^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/.test(tag) || /\s/.test(tag)) {
    throw new Error('镜像标签包含非法字符')
  }

  return {
    short,
    tag,
    image: `renewapi:${tag}`,
    product_version: input.version,
    release_tag: releaseTag,
    release_name: name,
    build_channel: channel,
    prerelease: String(prerelease),
    make_latest: String(product && !prerelease),
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  const metadata = releaseMetadata({
    sha: process.env.GITHUB_SHA || '',
    expectedSha: process.env.EXPECTED_SHA || '',
    ref: process.env.GITHUB_REF || '',
    runId: process.env.GITHUB_RUN_ID || '',
    version: fs.readFileSync('VERSION', 'utf8').trim(),
    imageTag: process.env.INPUT_IMAGE_TAG || '',
    releaseTag: process.env.INPUT_RELEASE_TAG || '',
  })
  metadata.created = new Date().toISOString()
  const output = Object.entries(metadata).map(([key, value]) => `${key}=${value}\n`).join('')
  fs.appendFileSync(process.env.GITHUB_OUTPUT, output)
  process.stdout.write(JSON.stringify(metadata, null, 2) + '\n')
}
