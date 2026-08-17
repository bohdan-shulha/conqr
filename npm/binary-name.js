export const targets = [
  ['darwin', 'x64'],
  ['darwin', 'arm64'],
  ['linux', 'x64'],
  ['linux', 'arm64'],
  ['win32', 'x64'],
  ['win32', 'arm64']
];

export function binaryName(platform, arch) {
  const goArch = arch === 'x64' ? 'amd64' : arch;
  return platform === 'win32'
    ? `conqr-${platform}-${goArch}.exe`
    : `conqr-${platform}-${goArch}`;
}
