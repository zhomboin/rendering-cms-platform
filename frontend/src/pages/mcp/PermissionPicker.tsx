import { Alert, Checkbox, Space, Typography } from 'antd';
import { permissionGroups } from '../../api/mcp';
import type { McpPermission } from '../../api/mcp';

export function PermissionPicker({
  value = [],
  onChange,
  id,
}: {
  value?: McpPermission[];
  onChange?: (value: McpPermission[]) => void;
  id?: string;
}) {
  return (
    <Space orientation="vertical" size={8} style={{ width: '100%' }}>
      <div role="group" aria-label="工具权限">
        <Space orientation="vertical" size={12} style={{ width: '100%' }}>
          {permissionGroups.map((group, groupIndex) => (
            <div key={group.label} role="group" aria-label={group.label}>
              <Typography.Text strong>{group.label}</Typography.Text>
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: 'repeat(auto-fit, minmax(min(100%, 9em), 1fr))',
                  gap: 6,
                }}
              >
                {group.options.map(({ value: name, label }, index) => (
                  <Checkbox
                    key={name}
                    id={groupIndex === 0 && index === 0 ? id : undefined}
                    checked={value.includes(name)}
                    onChange={(event) =>
                      onChange?.(event.target.checked ? [...value, name] : value.filter((p) => p !== name))
                    }
                  >
                    {label}
                  </Checkbox>
                ))}
              </div>
            </div>
          ))}
        </Space>
      </div>
      {value.includes('publish_article') && (
        <Alert type="warning" showIcon title="允许直接公开发布本连接创建的草稿。" />
      )}
    </Space>
  );
}
