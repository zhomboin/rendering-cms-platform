import { Avatar, Button, Dropdown, Layout, Menu, Typography } from 'antd';
import type { MenuProps } from 'antd';
import {
  DashboardOutlined,
  DownOutlined,
  FileTextOutlined,
  MessageOutlined,
  FolderOutlined,
  LogoutOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { useState } from 'react';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { clearAuthToken, getAuthToken, getAuthUser } from '../api/auth';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;
const logoSrc = '/logo/icon.png';

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
  const [collapsed, setCollapsed] = useState(false);
  const authToken = getAuthToken();
  const authUser = getAuthUser();

  if (!authToken) {
    return <Navigate to="/admin/login" replace state={{ from: location }} />;
  }

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
  const userName = authUser?.name || '管理员';
  const userEmail = authUser?.email || '当前账号';
  const userRole =
    authUser?.role === 'admin'
      ? '管理员'
      : authUser?.role === 'editor'
        ? '编辑'
        : authUser?.role || '已登录';
  const userInitial = userName.trim().slice(0, 1).toUpperCase() || 'A';

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'profile',
      label: (
        <div style={{ minWidth: 220, padding: '6px 4px' }}>
          <Text strong style={{ display: 'block', color: '#0F172A', marginBottom: 4 }}>
            {userName}
          </Text>
          <Text style={{ display: 'block', color: '#64748B', fontSize: 12, marginBottom: 4 }}>
            {userEmail}
          </Text>
          <Text style={{ color: '#4F46E5', fontSize: 12 }}>{userRole}</Text>
        </div>
      ),
    },
    { type: 'divider' },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
    },
  ];

  const handleUserMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key !== 'logout') return;
    clearAuthToken();
    navigate('/admin/login', { replace: true });
  };

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        width={240}
        theme="light"
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
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
            padding: collapsed ? '0 16px' : '0 24px',
          }}
        >
          <img
            src={logoSrc}
            alt="Rendering CMS"
            style={{
              display: 'block',
              width: collapsed ? 44 : 160,
              height: collapsed ? 44 : 52,
              objectFit: 'contain',
            }}
          />
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

          <Dropdown
            trigger={['click']}
            placement="bottomRight"
            menu={{ items: userMenuItems, onClick: handleUserMenuClick }}
          >
            <Button
              type="text"
              style={{
                height: 44,
                padding: '0 10px',
                borderRadius: 8,
                display: 'inline-flex',
                alignItems: 'center',
                lineHeight: 1,
              }}
            >
              <span
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 8,
                  minWidth: 0,
                  whiteSpace: 'nowrap',
                }}
              >
                <Avatar
                  size={28}
                  icon={!authUser?.name ? <UserOutlined /> : undefined}
                  style={{ backgroundColor: '#4F46E5', fontSize: 13 }}
                >
                  {authUser?.name ? userInitial : null}
                </Avatar>
                <span
                  style={{
                    display: 'inline-block',
                    maxWidth: 120,
                    minWidth: 0,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    lineHeight: '20px',
                    verticalAlign: 'middle',
                  }}
                >
                  {userName}
                </span>
                <DownOutlined
                  style={{ flex: '0 0 auto', fontSize: 12, color: '#64748B' }}
                />
              </span>
            </Button>
          </Dropdown>
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
