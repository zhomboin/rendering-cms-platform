import { Layout, Menu, Space, Typography } from 'antd';
import {
  DashboardOutlined,
  FileTextOutlined,
  MessageOutlined,
  FolderOutlined,
} from '@ant-design/icons';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;

const menuItems = [
  { key: '/admin', icon: <DashboardOutlined />, label: '仪表盘' },
  { key: '/admin/articles', icon: <FileTextOutlined />, label: '文章管理' },
  { key: '/admin/comments', icon: <MessageOutlined />, label: '评论审核' },
  { key: '/admin/assets', icon: <FolderOutlined />, label: '资源管理' },
];

const pageTitleMap: Record<string, string> = {
  '/admin': '仪表盘',
  '/admin/articles': '文章管理',
  '/admin/comments': '评论审核',
  '/admin/assets': '资源管理',
};

function AdminLayout() {
  const navigate = useNavigate();
  const location = useLocation();

  // Determine the deepest matching menu key for the current path.
  // This ensures sub-routes (e.g. /admin/articles/new) still highlight the
  // parent menu item.
  const selectedKey =
    menuItems
      .map((item) => item.key)
      .filter(
        (key) =>
          location.pathname === key ||
          location.pathname.startsWith(key + '/'),
      )
      .sort((a, b) => b.length - a.length)[0] ?? '/admin';

  const pageTitle = pageTitleMap[selectedKey] ?? '';

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        width={240}
        style={{
          background: '#FFFFFF',
          borderRight: '1px solid #E2E8F0',
        }}
      >
        <div
          style={{
            height: 80,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            borderBottom: '1px solid #E2E8F0',
          }}
        >
          <Text strong style={{ fontSize: 20, color: '#0F172A' }}>
            Rendering{'\u00A0'}
            <span style={{ color: '#4F46E5' }}>CMS</span>
          </Text>
        </div>

        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
          style={{ borderInlineEnd: 'none', marginTop: 8 }}
        />
      </Sider>

      <Layout>
        <Header
          style={{
            height: 80,
            lineHeight: '80px',
            background: '#FFFFFF',
            borderBottom: '1px solid #E2E8F0',
            padding: '0 24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <Text strong style={{ fontSize: 18, color: '#0F172A' }}>
            {pageTitle}
          </Text>

          {/* Reserved for auth */}
          <Space>
            <Text style={{ color: '#64748B' }}>管理员</Text>
          </Space>
        </Header>

        <Content
          style={{
            padding: 24,
            minHeight: 'calc(100vh - 80px)',
            background: '#F8FAFC',
          }}
        >
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}

export default AdminLayout;
