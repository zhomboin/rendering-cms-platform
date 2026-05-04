import React from 'react';
import ReactDOM from 'react-dom/client';
import 'antd/dist/reset.css';
import { AppProviders } from './app/providers';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppProviders />
  </React.StrictMode>,
);
