import React from 'react';
import ReactDOM from 'react-dom/client';
import 'antd/dist/reset.css';
import './app/theme.css';
import { AppProviders } from './app/providers';
import { applyCmsThemeColors } from './app/theme-colors';

applyCmsThemeColors(document.documentElement);

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <AppProviders />
  </React.StrictMode>,
);
