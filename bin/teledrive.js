#!/usr/bin/env node

/**
 * TeleDrive CLI / Server Runner
 * Official Node.js binary wrapper for TeleDrive
 * Zero external dependencies.
 */

const { spawnSync } = require('node:child_process');
const fs = require('node:fs');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');

const VERSION = '1.6.0';
const BINARY_VERSION = '1.6.0';

// Map process.platform to TeleDrive release platform name
const PLATFORM_MAP = {
  linux: 'linux',
  darwin: 'darwin',
  win32: 'windows'
};

// Map process.arch to TeleDrive release architecture name
const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64'
};

function getBinaryName() {
  return process.platform === 'win32' ? 'teledrive.exe' : 'teledrive';
}

function getCacheDir() {
  const custom = process.env.TELEDRIVE_CACHE_DIR;
  if (custom) return custom;

  if (process.platform === 'win32') {
    return path.join(process.env.LOCALAPPDATA || os.homedir(), 'teledrive', 'bin');
  }
  return path.join(os.homedir(), '.cache', 'teledrive', 'bin');
}

function findLocalBinary() {
  // 1. Explicit environment override
  if (process.env.TELEDRIVE_BINARY_PATH && fs.existsSync(process.env.TELEDRIVE_BINARY_PATH)) {
    return process.env.TELEDRIVE_BINARY_PATH;
  }

  // 2. Local cache directory
  const cached = path.join(getCacheDir(), `teledrive-v${BINARY_VERSION}${process.platform === 'win32' ? '.exe' : ''}`);
  if (fs.existsSync(cached)) {
    return cached;
  }

  // 3. Local repo root binary (if running inside git clone)
  const repoBin = path.join(__dirname, '..', getBinaryName());
  if (fs.existsSync(repoBin)) {
    return repoBin;
  }

  return null;
}

function downloadUrl(url, destPath) {
  return new Promise((resolve, reject) => {
    const options = {
      headers: {
        'User-Agent': `teledrive-npm-wrapper/${VERSION}`
      }
    };
    if (process.env.GITHUB_TOKEN) {
      options.headers['Authorization'] = `Bearer ${process.env.GITHUB_TOKEN}`;
    }

    const request = https.get(url, options, (res) => {
      // Handle redirects (GitHub Releases redirect to AWS S3/CDN)
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        return downloadUrl(res.headers.location, destPath).then(resolve).catch(reject);
      }

      if (res.statusCode !== 200) {
        return reject(new Error(`Server returned HTTP ${res.statusCode} (${res.statusMessage})`));
      }

      const fileStream = fs.createWriteStream(destPath);
      res.pipe(fileStream);
      fileStream.on('finish', () => {
        fileStream.close(() => resolve(destPath));
      });
      fileStream.on('error', (err) => {
        fs.unlink(destPath, () => reject(err));
      });
    });

    request.on('error', (err) => {
      fs.unlink(destPath, () => reject(err));
    });
  });
}

function extractArchive(archivePath, destDir, platform) {
  if (platform === 'windows') {
    // 1. Try Windows tar (tar -xf handles .zip on Windows 10/11)
    const tarResult = spawnSync('tar', ['-xf', archivePath, '-C', destDir], { stdio: 'inherit' });
    if (tarResult.status === 0) return;

    // 2. Fallback to PowerShell Expand-Archive (built into all modern Windows systems)
    const psCmd = `Expand-Archive -LiteralPath '${archivePath}' -DestinationPath '${destDir}' -Force`;
    const psResult = spawnSync('powershell.exe', ['-NoProfile', '-ExecutionPolicy', 'Bypass', '-Command', psCmd], { stdio: 'inherit' });
    if (psResult.status === 0) return;

    throw new Error(`Failed to extract Windows archive via tar or PowerShell.`);
  }

  // Linux and macOS: tar -xzf
  const extractResult = spawnSync('tar', ['-xzf', archivePath, '-C', destDir], { stdio: 'inherit' });
  if (extractResult.status !== 0) {
    throw new Error(`tar extraction failed with exit code ${extractResult.status}`);
  }
}

async function ensureBinary() {
  const existing = findLocalBinary();
  if (existing) return existing;

  const platform = PLATFORM_MAP[process.platform];
  const arch = ARCH_MAP[process.arch];

  if (!platform || !arch) {
    throw new Error(`Unsupported platform or architecture: ${process.platform}-${process.arch}`);
  }

  const cacheDir = getCacheDir();
  fs.mkdirSync(cacheDir, { recursive: true });

  const binTarget = path.join(cacheDir, `teledrive-v${BINARY_VERSION}${process.platform === 'win32' ? '.exe' : ''}`);
  const ext = (platform === 'windows') ? '.zip' : '.tar.gz';
  const archiveName = `teledrive-v${BINARY_VERSION}-${platform}-${arch}${ext}`;
  const downloadArchive = path.join(cacheDir, archiveName);
  const releaseUrl = `https://github.com/herliansyah/teledrive/releases/download/v${BINARY_VERSION}/${archiveName}`;

  console.log(`[teledrive] Binary not found locally. Downloading TeleDrive v${BINARY_VERSION} for ${platform}/${arch}...`);

  try {
    await downloadUrl(releaseUrl, downloadArchive);

    extractArchive(downloadArchive, cacheDir, platform);

    // Clean up archive
    try { fs.unlinkSync(downloadArchive); } catch (_) {}

    // In archive, the binary is named 'teledrive' (or 'teledrive.exe')
    const extractedBin = path.join(cacheDir, getBinaryName());
    if (extractedBin !== binTarget && fs.existsSync(extractedBin)) {
      fs.renameSync(extractedBin, binTarget);
    }

    if (process.platform !== 'win32') {
      fs.chmodSync(binTarget, 0o755);
    }

    console.log(`[teledrive] Successfully cached binary at: ${binTarget}`);
    return binTarget;
  } catch (err) {
    // Attempt build fallback if go is installed
    console.warn(`[teledrive] Download failed: ${err.message}`);
    const goCheck = spawnSync('go', ['version'], { stdio: 'pipe' });
    if (goCheck.status === 0) {
      console.log(`[teledrive] Go compiler detected. Attempting to build binary locally...`);
      const repoRoot = path.join(__dirname, '..');
      const buildRes = spawnSync('go', ['build', '-o', binTarget, './cmd/teledrive'], {
        cwd: repoRoot,
        stdio: 'inherit'
      });
      if (buildRes.status === 0 && fs.existsSync(binTarget)) {
        console.log(`[teledrive] Local build succeeded!`);
        return binTarget;
      }
    }

    throw new Error(
      `Could not obtain TeleDrive binary for ${platform}-${arch}.\n` +
      `URL: ${releaseUrl}\n` +
      `Details: ${err.message}\n\n` +
      `You can manually place the binary at:\n  ${binTarget}\n` +
      `Or set TELEDRIVE_BINARY_PATH=/path/to/teledrive`
    );
  }
}

async function main() {
  try {
    const binPath = await ensureBinary();
    const args = process.argv.slice(2);
    const result = spawnSync(binPath, args, { stdio: 'inherit' });
    process.exit(result.status ?? 0);
  } catch (err) {
    console.error(`[teledrive error] ${err.message}`);
    process.exit(1);
  }
}

main();
