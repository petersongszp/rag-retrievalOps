'use client';

import { Layout, Typography } from 'antd';
import type { FC } from 'react';


const { Footer: AntFooter } = Layout;
const { Text } = Typography;

const Footer: FC = () => {
  

  return (
    <AntFooter className="bg-white border-t py-6">
      <div className="container mx-auto px-4 text-center">
        <Text className="text-gray-600">{"© 2024 Interview Master AI"}</Text>
        <div className="mt-2 space-x-4">
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            {"Privacy Policy"}
          </a>
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            {"Terms of Service"}
          </a>
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            {"Contact Us"}
          </a>
        </div>
      </div>
    </AntFooter>
  );
};

export default Footer;
