import { Avatar, Button, Drawer, Dropdown, Grid, Layout, Menu, Typography } from 'antd';
import type { MenuProps } from 'antd';
import {
  DashboardOutlined,
  DownOutlined,
  FileTextOutlined,
  FolderOutlined,
  LogoutOutlined,
  MenuOutlined,
  MessageOutlined,
  PictureOutlined,
  SendOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { useState } from 'react';
import { Link, Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { clearAuthToken, getAuthToken, getAuthUser } from '../api/auth';
import './AdminLayout.css';

const { Header, Sider, Content } = Layout;
const { Text } = Typography;
const { useBreakpoint } = Grid;

const baseMenuItems = [
  { key: '/admin', icon: <DashboardOutlined />, label: '仪表盘' },
  { key: '/admin/articles', icon: <FileTextOutlined />, label: '文章管理' },
  { key: '/admin/albums', icon: <PictureOutlined />, label: '相册管理' },
  { key: '/admin/comments', icon: <MessageOutlined />, label: '评论审核' },
  { key: '/admin/assets', icon: <FolderOutlined />, label: '资源管理' },
];

const postBridgeItem = { key: '/admin/postbridge', icon: <SendOutlined />, label: 'PostBridge' };
const brandIcon = (
  <span className="cms-brand-icon">
    <img src="/logo/rendering-woodcut-128.png" width={30} height={30} alt="" />
  </span>
);

export default function AdminLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const [navPreference, setNavPreference] = useState<boolean>();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const screens = useBreakpoint();
  const isMobile = !screens.md;
  const collapsed = navPreference ?? !screens.lg;
  const authToken = getAuthToken();
  const authUser = getAuthUser();

  if (!authToken) {
    return <Navigate to="/admin/login" replace state={{ from: location }} />;
  }

  const menuItems = authUser?.role === 'admin' ? [...baseMenuItems, postBridgeItem] : baseMenuItems;
  const activeItem =
    menuItems
      .filter(({ key }) => location.pathname === key || location.pathname.startsWith(key + '/'))
      .sort((a, b) => b.key.length - a.key.length)[0] ?? baseMenuItems[0];

  const userName = authUser?.name || '管理员';
  const userEmail = authUser?.email || '当前账号';
  const role = authUser?.role;
  const userRole = role === 'admin' ? '管理员' : role === 'editor' ? '编辑' : role || '已登录';
  const userInitial = Array.from(userName.trim())[0]?.toUpperCase() || 'A';

  const userMenuItems: MenuProps['items'] = [
    {
      key: 'profile',
      disabled: true,
      label: (
        <div className="cms-account-details">
          <Text strong>{userName}</Text>
          <Text type="secondary">{userEmail}</Text>
          <small>{userRole}</small>
        </div>
      ),
    },
    { type: 'divider' },
    { key: 'logout', icon: <LogoutOutlined />, label: '退出登录' },
  ];

  const handleUserMenuClick: MenuProps['onClick'] = ({ key }) => {
    if (key !== 'logout') return;
    clearAuthToken();
    navigate('/admin/login', { replace: true });
  };

  const navMenu = (drawer = false) => (
    <nav aria-label={drawer ? '移动端后台导航' : '后台导航'}>
      <Menu
        className="cms-nav"
        mode="inline"
        selectedKeys={[activeItem.key]}
        items={menuItems}
        onClick={({ key }) => {
          navigate(key);
          setMobileMenuOpen(false);
        }}
      />
    </nav>
  );

  return (
    <Layout className="cms-shell">
      {!isMobile && (
        <Sider
          className="cms-sidebar"
          width={224}
          collapsedWidth={68}
          theme="light"
          collapsible
          collapsed={collapsed}
          onCollapse={setNavPreference}
          trigger={null}
        >
          <Link to="/admin" className="cms-brand" aria-label="Rendering CMS 仪表盘">
            {brandIcon}
            {!collapsed && <strong>Rendering CMS</strong>}
          </Link>
          {navMenu()}
          <Button
            className="cms-collapse"
            type="text"
            aria-expanded={!collapsed}
            aria-label={collapsed ? '展开导航' : '收起导航'}
            onClick={() => setNavPreference(!collapsed)}
            icon={<MenuOutlined />}
          />
        </Sider>
      )}

      <Layout className="cms-main">
        <Header className="cms-header">
          <div className="cms-header-start">
            {isMobile && (
              <Button
                aria-label="打开导航"
                aria-expanded={mobileMenuOpen}
                size="large"
                icon={<MenuOutlined />}
                onClick={() => setMobileMenuOpen(true)}
              />
            )}
            <Text strong ellipsis>
              {activeItem.label}
            </Text>
          </div>
          <Dropdown
            trigger={['click']}
            placement="bottomRight"
            menu={{ items: userMenuItems, onClick: handleUserMenuClick }}
          >
            <Button type="text" className="cms-account" aria-label={`账号菜单：${userName}`}>
              <Avatar size={28} icon={!authUser?.name ? <UserOutlined /> : undefined}>
                {authUser?.name ? userInitial : null}
              </Avatar>
              <span className="cms-account-name">{userName}</span>
              <DownOutlined aria-hidden="true" />
            </Button>
          </Dropdown>
        </Header>

        <Drawer
          title={
            <span className="cms-drawer-brand">
              {brandIcon}
              Rendering CMS
            </span>
          }
          placement="left"
          open={isMobile && mobileMenuOpen}
          onClose={() => setMobileMenuOpen(false)}
          size="min(280px, calc(100vw - 24px))"
          styles={{ body: { padding: 8 } }}
        >
          {navMenu(true)}
        </Drawer>

        <Content className="cms-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}
