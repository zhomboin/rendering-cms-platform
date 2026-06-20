import { lazy, Suspense } from 'react';
import { Spin } from 'antd';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import AdminLayout from '../layouts/AdminLayout';

const AssetsPage = lazy(() => import('../pages/assets/AssetsPage'));
const ArticleEditorPage = lazy(() => import('../pages/articles/ArticleEditorPage'));
const ArticleListPage = lazy(() => import('../pages/articles/ArticleListPage'));
const CommentsPage = lazy(() => import('../pages/comments/CommentsPage'));
const DashboardPage = lazy(() => import('../pages/dashboard/DashboardPage'));
const LoginPage = lazy(() => import('../pages/auth/LoginPage'));

function RouteFallback() {
  return (
    <div style={{ minHeight: 240, display: 'grid', placeItems: 'center' }}>
      <Spin />
    </div>
  );
}

export function AppRoutes() {
  return (
    <BrowserRouter>
      <Suspense fallback={<RouteFallback />}>
        <Routes>
          <Route index element={<Navigate to="/admin" replace />} />
          <Route path="/admin/login" element={<LoginPage />} />
          <Route path="/admin" element={<AdminLayout />}>
            <Route index element={<DashboardPage />} />
            <Route path="articles" element={<ArticleListPage />} />
            <Route path="articles/new" element={<ArticleEditorPage />} />
            <Route path="articles/:id/edit" element={<ArticleEditorPage />} />
            <Route path="comments" element={<CommentsPage />} />
            <Route path="assets" element={<AssetsPage />} />
          </Route>
        </Routes>
      </Suspense>
    </BrowserRouter>
  );
}
