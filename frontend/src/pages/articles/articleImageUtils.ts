import type { AssetFile } from '../../api/assets';
import type { AlbumPhoto } from '../../api/albums';

export interface ArticleImage {
  url: string;
  alt: string;
}

const imageTypes = new Set(['image/png', 'image/jpeg', 'image/webp']);

export function isPermanentPublicImageUrl(value: unknown): value is string {
  if (typeof value !== 'string' || !value.trim() || /[\u0000-\u001f\u007f]/.test(value)) return false;
  try {
    const url = new URL(value);
    if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password) return false;
    for (const key of url.searchParams.keys()) {
      if (/^(x-amz-|x-goog-)/i.test(key) || /^(AWSAccessKeyId|GoogleAccessId|Signature|Expires)$/i.test(key)) return false;
    }
    return true;
  } catch {
    return false;
  }
}

export function assetToImage(asset: AssetFile): ArticleImage | null {
  if (asset.status !== 'active' || asset.deletedAt !== null || !imageTypes.has(asset.contentType) || !isPermanentPublicImageUrl(asset.publicUrl)) return null;
  return { url: asset.publicUrl.trim(), alt: asset.filename };
}

export function photoToImage(photo: AlbumPhoto): ArticleImage | null {
  if (!imageTypes.has(photo.contentType) || !isPermanentPublicImageUrl(photo.url)) return null;
  return { url: photo.url.trim(), alt: photo.alt || photo.filename };
}

export function imageUrlKey(url: string) {
  try { return new URL(url).href; } catch { return url; }
}

export function uniqueArticleImages(images: ArticleImage[]) {
  const seen = new Set<string>();
  return images.filter(image => {
    const key = imageUrlKey(image.url);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function imageMarkdown(image: ArticleImage) {
  const alt = image.alt.replace(/[\r\n\u2028\u2029]+/g, ' ').replace(/([\\[\]\x60*_{}<>])/g, '\\$1');
  const url = image.url.trim().replace(/[\s()<>"\\\u0000-\u001f\u007f]/g, char => char === '(' ? '%28' : char === ')' ? '%29' : encodeURIComponent(char));
  return '![' + alt + '](' + url + ')';
}

export function insertArticleImages(value: string, start: number, end: number, images: ArticleImage[]) {
  const before = value.slice(0, start);
  const after = value.slice(end);
  const prefix = before.trimEnd() ? before.endsWith('\n\n') ? '' : before.endsWith('\n') ? '\n' : '\n\n' : '';
  const suffix = after.trimStart() ? after.startsWith('\n\n') ? '' : after.startsWith('\n') ? '\n' : '\n\n' : '';
  const text = prefix + uniqueArticleImages(images).map(imageMarkdown).join('\n\n') + suffix;
  return { value: before + text + after, cursor: start + text.length };
}
