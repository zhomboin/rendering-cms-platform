import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';

function DashboardPlaceholder() {
  return (
    <main style={{ padding: 24 }}>
      <h1>Rendering CMS Platform</h1>
      <p>后台基础壳层已就绪，后续阶段将接入仪表盘、文章、评论和资源管理。</p>
    </main>
  );
}

export function AppRoutes() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/admin" replace />} />
        <Route path="/admin" element={<DashboardPlaceholder />} />
      </Routes>
    </BrowserRouter>
  );
}
