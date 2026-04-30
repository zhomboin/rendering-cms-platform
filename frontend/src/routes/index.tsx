import { BrowserRouter, Route, Routes } from 'react-router-dom';
import AdminLayout from '../components/AdminLayout';
import LoginPage from '../features/auth/LoginPage';
import ArticleListPage from '../features/articles/ArticleListPage';
import ArticleDetailPage from '../features/articles/ArticleDetailPage';
import AdminArticleListPage from '../features/articles/AdminArticleListPage';
import AdminArticleEditorPage from '../features/articles/AdminArticleEditorPage';
import AdminDashboardPage from '../features/analytics/AdminDashboardPage';
import AdminCommentsPage from '../features/comments/AdminCommentsPage';
import AdminAssetsPage from '../features/assets/AdminAssetsPage';

export function AppRoutes() {
  return (
    <BrowserRouter>
      <Routes>
        <Route index element={<ArticleListPage />} />
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
