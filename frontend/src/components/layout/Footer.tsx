'use client';

import { Layout, Typography } from 'antd';
import type { FC } from 'react';

const { Footer: AntFooter } = Layout;
const { Text } = Typography;

const Footer: FC = () => {
  return (
    <AntFooter className="bg-white border-t py-6">
      <div className="container mx-auto px-4 text-center">
        <Text className="text-gray-600">© 2024 面试吧AI面试平台</Text>
        <div className="mt-2 space-x-4">
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            隐私政策
          </a>
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            服务条款
          </a>
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            联系我们
          </a>
        </div>
      </div>
    </AntFooter>
  );
};

export default Footer;
