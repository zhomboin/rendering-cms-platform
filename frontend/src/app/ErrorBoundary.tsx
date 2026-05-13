import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Alert, Button, Result } from 'antd';

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = {
    error: null,
  };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('admin route render failed', error, errorInfo);
  }

  render() {
    if (!this.state.error) {
      return this.props.children;
    }

    return (
      <Result
        status="error"
        title="页面加载失败"
        subTitle="当前页面发生异常，请刷新后重试。"
        extra={[
          <Button type="primary" key="reload" onClick={() => window.location.reload()}>
            刷新页面
          </Button>,
        ]}
      >
        <Alert
          type="error"
          showIcon
          message="错误详情"
          description={this.state.error.message || '未知错误'}
        />
      </Result>
    );
  }
}
