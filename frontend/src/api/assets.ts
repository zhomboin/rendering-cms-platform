import axios from 'axios';
import { apiGet, apiPost } from './client';

export const allowedAssetContentTypes = [
  'image/png',
  'image/jpeg',
  'image/webp',
  'application/pdf',
  'text/plain',
  'application/zip',
];

export interface AssetFile {
  assetId: string;
  filename: string;
  contentType: string;
  byteSize: number;
  publicUrl: string | null;
  createdBy: string;
  createdAt: string;
}

interface UploadURLResponse {
  asset: AssetFile;
  uploadUrl: string;
  method: 'PUT';
  headers: Record<string, string>;
  expiresInSeconds: number;
}

interface DownloadURLResponse {
  asset: AssetFile;
  downloadUrl: string;
  expiresInSeconds: number;
}

export function listAdminAssets() {
  return apiGet<AssetFile[]>('/admin/assets');
}

export async function getAdminAssetDownloadUrl(assetId: string) {
  const response = await apiGet<DownloadURLResponse>(`/admin/assets/${assetId}/download-url`);
  return response.downloadUrl;
}

export async function uploadAdminAsset(file: File) {
  const upload = await apiPost<UploadURLResponse>('/admin/assets/upload-url', {
    filename: file.name,
    contentType: file.type,
    byteSize: file.size,
  });
  await axios.put(upload.uploadUrl, file, {
    headers: upload.headers,
    withCredentials: false,
  });
  return upload.asset;
}
