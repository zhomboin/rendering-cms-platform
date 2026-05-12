import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import AdminLayout from '../layouts/AdminLayout';
import AssetsPage from '../pages/assets/AssetsPage';
import ArticleEditorPage from '../pages/articles/ArticleEditorPage';
import ArticleListPage from '../pages/articles/ArticleListPage';
import LoginPage from '../pages/auth/LoginPage';
import CommentsPage from '../pages/comments/CommentsPage';
import DashboardPage from '../pages/dashboard/DashboardPage';

export function AppRoutes() {
  return (
    <BrowserRouter>
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
    </BrowserRouter>
  );
}
