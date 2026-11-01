export const imageUploadBytes = 50 * 1024 * 1024;
export const fileUploadBytes = 20 * 1024 * 1024;
export const imageContentTypes = ['image/png', 'image/jpeg', 'image/webp'];

export function validateUploadSize(file: Pick<File, 'type' | 'size'>): void {
  const image = imageContentTypes.includes(file.type);
  const limit = image ? imageUploadBytes : fileUploadBytes;
  if (file.size <= 0 || file.size > limit) {
    throw new Error(`${image ? '图片' : '文件'}大小必须大于 0 且不超过 ${image ? 50 : 20}MB`);
  }
}
