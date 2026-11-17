const kibibyte = 1024;
const mebibyte = kibibyte * 1024;
const gibibyte = mebibyte * 1024;

export function formatFileSize(bytes: number): string {
  if (bytes >= gibibyte) return `${(bytes / gibibyte).toFixed(1)} GiB`;
  if (bytes >= mebibyte) return `${(bytes / mebibyte).toFixed(1)} MiB`;
  if (bytes >= kibibyte) return `${(bytes / kibibyte).toFixed(1)} KiB`;
  return `${bytes} B`;
}
