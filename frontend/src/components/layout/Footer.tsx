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
      </div>
    </AntFooter>
  );
};

export default Footer;
