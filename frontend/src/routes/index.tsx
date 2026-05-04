import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import AdminLayout from '../layouts/AdminLayout';
import AdminDashboardPage from '../pages/admin/analytics/AdminDashboardPage';
import AdminArticleEditorPage from '../pages/admin/articles/AdminArticleEditorPage';
import AdminArticleListPage from '../pages/admin/articles/AdminArticleListPage';
import AdminAssetsPage from '../pages/admin/assets/AdminAssetsPage';
import AdminCommentsPage from '../pages/admin/comments/AdminCommentsPage';
import LoginPage from '../pages/auth/LoginPage';
import ArticleDetailPage from '../pages/public/articles/ArticleDetailPage';
import ArticleListPage from '../pages/public/articles/ArticleListPage';

export function AppRoutes() {
  return (
    <BrowserRouter>
      <Routes>
        <Route index element={<Navigate to="/admin" replace />} />
        <Route path="/articles" element={<ArticleListPage />} />
        <Route path="/articles/:slug" element={<ArticleDetailPage />} />

        <Route path="/admin/login" element={<LoginPage />} />
        <Route path="/admin" element={<AdminLayout />}>
          <Route index element={<AdminDashboardPage />} />
          <Route path="articles" element={<AdminArticleListPage />} />
          <Route path="articles/new" element={<AdminArticleEditorPage />} />
          <Route path="articles/:id/edit" element={<AdminArticleEditorPage />} />
          <Route path="comments" element={<AdminCommentsPage />} />
          <Route path="assets" element={<AdminAssetsPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
