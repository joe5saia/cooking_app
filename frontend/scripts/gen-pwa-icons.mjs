import fs from 'node:fs'
import path from 'node:path'
import { deflateSync } from 'node:zlib'

function crc32(buf) {
  const table = new Uint32Array(256)
  for (let i = 0; i < 256; i++) {
    let c = i
    for (let k = 0; k < 8; k++) {
      c = (c & 1) !== 0 ? 0xedb88320 ^ (c >>> 1) : c >>> 1
    }
    table[i] = c >>> 0
  }

  let crc = 0xffffffff
  for (const b of buf) {
    crc = table[(crc ^ b) & 0xff] ^ (crc >>> 8)
  }
  return (crc ^ 0xffffffff) >>> 0
}

function u32(n) {
  const b = Buffer.alloc(4)
  b.writeUInt32BE(n >>> 0, 0)
  return b
}

function chunk(type, data) {
  const typeBuf = Buffer.from(type, 'ascii')
  const len = u32(data.length)
  const crc = u32(crc32(Buffer.concat([typeBuf, data])))
  return Buffer.concat([len, typeBuf, data, crc])
}

function solidPng({ width, height, rgba }) {
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10])

  const ihdr = Buffer.alloc(13)
  ihdr.writeUInt32BE(width, 0)
  ihdr.writeUInt32BE(height, 4)
  ihdr.writeUInt8(8, 8) // bit depth
  ihdr.writeUInt8(6, 9) // color type (RGBA)
  ihdr.writeUInt8(0, 10) // compression
  ihdr.writeUInt8(0, 11) // filter
  ihdr.writeUInt8(0, 12) // interlace

  const bytesPerPixel = 4
  const rowBytes = width * bytesPerPixel
  const raw = Buffer.alloc((rowBytes + 1) * height)

  for (let y = 0; y < height; y++) {
    const rowStart = y * (rowBytes + 1)
    raw[rowStart] = 0 // filter: none
    for (let x = 0; x < width; x++) {
      const p = rowStart + 1 + x * bytesPerPixel
      raw[p + 0] = rgba[0]
      raw[p + 1] = rgba[1]
      raw[p + 2] = rgba[2]
      raw[p + 3] = rgba[3]
    }
  }

  const compressed = deflateSync(raw, { level: 9 })

  return Buffer.concat([
    signature,
    chunk('IHDR', ihdr),
    chunk('IDAT', compressed),
    chunk('IEND', Buffer.alloc(0)),
  ])
}

function writeIcon({ outDir, name, size, rgba }) {
  const buf = solidPng({ width: size, height: size, rgba })
  fs.writeFileSync(path.join(outDir, name), buf)
}

const outDir = path.resolve(process.cwd(), 'public')
fs.mkdirSync(outDir, { recursive: true })

// Dark background to match app shell.
const bg = [11, 18, 32, 255]

writeIcon({ outDir, name: 'pwa-192.png', size: 192, rgba: bg })
writeIcon({ outDir, name: 'pwa-512.png', size: 512, rgba: bg })

console.log('generated icons in', outDir)
