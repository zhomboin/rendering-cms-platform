import { Radio, Space, Typography } from 'antd';
import { readScopeLabels } from '../../api/mcp';
import type { McpReadScope } from '../../api/mcp';

export function ReadScopePicker({
  id,
  value,
  onChange,
}: {
  id?: string;
  value?: McpReadScope;
  onChange?: (value: McpReadScope) => void;
}) {
  return (
    <Space orientation="vertical" size={4}>
      <Radio.Group
        id={id}
        aria-label="数据可见范围"
        value={value}
        onChange={(event) => onChange?.(event.target.value)}
      >
        <Radio value="own">{readScopeLabels.own}</Radio>
        <Radio value="all">{readScopeLabels.all}</Radio>
      </Radio.Group>
      {value === 'all' && (
        <Typography.Text type="secondary">
          可查看后台和其他连接的内容。私有文章仍受保护，写入仅限本连接创建的内容。
        </Typography.Text>
      )}
    </Space>
  );
}
