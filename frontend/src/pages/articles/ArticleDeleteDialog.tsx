import { useState, type KeyboardEvent } from 'react';
import { Alert, App, Modal } from 'antd';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { deleteAdminArticle, type AdminArticleRecord } from '../../api/articles';
import { ApiRequestError } from '../../api/client';
import { clearArticleDraft, getArticleDraftKey } from './articleEditorDraft';

export type DeleteTarget = { articleId: string; title: string; version: number };

export function ArticleDeleteDialog({
  target,
  onClose,
}: {
  target: DeleteTarget;
  onClose: () => void;
}) {
  const [open, setOpen] = useState(true);
  const { message } = App.useApp();
  const queryClient = useQueryClient();

  function removeFromCache() {
    clearArticleDraft(getArticleDraftKey(target.articleId));
    queryClient.removeQueries({ queryKey: ['article-revisions', target.articleId] });
    queryClient.removeQueries({ queryKey: ['article-revision', target.articleId] });
    queryClient.setQueryData<AdminArticleRecord[]>(['admin-articles'], (rows) =>
      rows?.filter((row) => row.articleId !== target.articleId),
    );
    for (const key of [
      'admin-articles',
      'admin-comments',
      'dashboard-overview',
      'dashboard-articles',
      'dashboard-trend',
      'postbridge-attempts',
      'postbridge-attempt',
    ]) {
      void queryClient.invalidateQueries({ queryKey: [key] });
    }
  }

  const deletion = useMutation({
    mutationFn: () => deleteAdminArticle(target.articleId, target.version),
    onSuccess: () => {
      removeFromCache();
      message.success('文章已删除');
      setOpen(false);
    },
    onError: (error) => {
      if (error instanceof ApiRequestError && error.status === 404) {
        removeFromCache();
        message.info('文章已不存在');
        setOpen(false);
      }
      if (error instanceof ApiRequestError && error.status === 409) {
        void queryClient.invalidateQueries({ queryKey: ['admin-articles'] });
        message.warning('文章已变更，请核对最新内容后再删除');
        setOpen(false);
      }
    },
  });

  return (
    <Modal
      open={open}
      title="永久删除文章？"
      okText="删除文章"
      okButtonProps={{ danger: true }}
      cancelText="取消"
      cancelButtonProps={{ autoFocus: true, disabled: deletion.isPending }}
      confirmLoading={deletion.isPending}
      onOk={() => {
        if (!deletion.isPending) deletion.mutate();
      }}
      onCancel={() => {
        if (!deletion.isPending) setOpen(false);
      }}
      wrapProps={{
        onKeyDown: (event: KeyboardEvent<HTMLDivElement>) => {
          if (event.key === 'Escape' && !event.nativeEvent.isComposing) {
            event.stopPropagation();
            if (!deletion.isPending) setOpen(false);
          }
        },
      }}
      afterClose={onClose}
    >
      <p style={{ overflowWrap: 'anywhere' }}>
        《{target.title}》及其修订、评论和访问记录将一并删除，无法在后台恢复。
      </p>
      {deletion.isError && open && <Alert type="error" showIcon title={deletion.error.message} />}
    </Modal>
  );
}
