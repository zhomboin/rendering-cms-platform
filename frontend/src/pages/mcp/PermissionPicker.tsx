import { Alert, Checkbox, Space } from 'antd';
import { permissionOptions } from '../../api/mcp';
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
    <Space orientation="vertical" size={8}>
      <div role="group" aria-label="工具权限">
        <Space orientation="vertical">
          {permissionOptions.map(({ value: name, label }, index) => (
            <Checkbox
              key={name}
              id={index === 0 ? id : undefined}
              checked={value.includes(name)}
              onChange={(event) =>
                onChange?.(event.target.checked ? [...value, name] : value.filter((p) => p !== name))
              }
            >
              {label}
            </Checkbox>
          ))}
        </Space>
      </div>
      {value.includes('publish_article') && (
        <Alert type="warning" showIcon title="允许直接公开发布本连接创建的草稿。" />
      )}
    </Space>
  );
}
