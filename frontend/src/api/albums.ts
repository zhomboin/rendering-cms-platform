import { apiClient, apiDelete, apiGet, apiPatch, apiPost, apiPut } from './client';

export type AlbumStatus = 'draft' | 'published';

export interface AdminAlbum {
  albumId: string;
  slug: string;
  title: string;
  titleEn: string | null;
  description: string;
  descriptionEn: string | null;
  status: AlbumStatus;
  tags: string[];
  coverPhotoId: string | null;
  publishedAt: string | null;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  photoCount: number;
  coverUrl: string | null;
  coverWidth: number | null;
  coverHeight: number | null;
}

export interface AlbumPhoto {
  photoId: string;
  alt: string;
  caption: string | null;
  width: number;
  height: number;
  takenAt: string | null;
  position: number;
  createdAt: string;
  url: string | null;
  filename: string;
  contentType: string;
  byteSize: number;
}

export interface AlbumInput {
  slug: string;
  title: string;
  titleEn?: string;
  description: string;
  descriptionEn?: string;
  tags?: string[];
}

export interface AlbumPhotoInput {
  alt: string;
  caption?: string;
  takenAt?: string | null;
}

export interface AlbumPhotoUploadOptions {
  signal?: AbortSignal;
  onProgress?: (progress: { loaded: number; total?: number }) => void;
}

export const photoContentTypes = ['image/png', 'image/jpeg', 'image/webp'];

export function listAdminAlbums() {
  return apiGet<AdminAlbum[]>('/admin/albums');
}

export function getAdminAlbum(albumId: string) {
  return apiGet<AdminAlbum>(`/admin/albums/${albumId}`);
}

export function createAdminAlbum(input: AlbumInput) {
  return apiPost<AdminAlbum>('/admin/albums', input);
}

export function updateAdminAlbum(albumId: string, input: AlbumInput) {
  return apiPatch<AdminAlbum>(`/admin/albums/${albumId}`, input);
}

export function publishAdminAlbum(albumId: string) {
  return apiPost<AdminAlbum>(`/admin/albums/${albumId}/publish`);
}

export function unpublishAdminAlbum(albumId: string) {
  return apiPost<AdminAlbum>(`/admin/albums/${albumId}/unpublish`);
}

export function deleteAdminAlbum(albumId: string) {
  return apiDelete<void>(`/admin/albums/${albumId}`);
}

export function listAdminAlbumPhotos(albumId: string) {
  return apiGet<AlbumPhoto[]>(`/admin/albums/${albumId}/photos`);
}

// 照片为后端代理上传：multipart 直传，默认 alt 取文件名（去扩展名），上传后可再编辑。
export async function uploadAdminAlbumPhoto(albumId: string, file: File, input: AlbumPhotoInput, options: AlbumPhotoUploadOptions = {}) {
  const form = new FormData();
  form.append('file', file);
  form.append('alt', input.alt);
  if (input.caption) {
    form.append('caption', input.caption);
  }
  if (input.takenAt) {
    form.append('takenAt', input.takenAt);
  }
  // 照片最大 20MB，放宽默认超时。
  const response = await apiClient.post<AlbumPhoto>(`/admin/albums/${albumId}/photos`, form, {
    // 清除 JSON 默认值，由浏览器生成 multipart boundary。
    headers: { 'Content-Type': undefined },
    timeout: 120000,
    signal: options.signal,
    onUploadProgress: event => options.onProgress?.({
      loaded: event.loaded,
      total: event.lengthComputable ? event.total : undefined,
    }),
  });
  return response.data;
}

export function updateAdminAlbumPhoto(albumId: string, photoId: string, input: AlbumPhotoInput) {
  return apiPatch<AlbumPhoto>(`/admin/albums/${albumId}/photos/${photoId}`, input);
}

export function deleteAdminAlbumPhoto(albumId: string, photoId: string) {
  return apiDelete<void>(`/admin/albums/${albumId}/photos/${photoId}`);
}

export function reorderAdminAlbumPhotos(albumId: string, photoIds: string[]) {
  return apiPut<AlbumPhoto[]>(`/admin/albums/${albumId}/photos/order`, { photoIds });
}

export function setAdminAlbumCover(albumId: string, photoId: string | null) {
  return apiPost<AdminAlbum>(`/admin/albums/${albumId}/cover`, { photoId });
}
