'use client';

import { Layout, Typography } from 'antd';
import type { FC } from 'react';
import { useTranslations } from 'next-intl';

const { Footer: AntFooter } = Layout;
const { Text } = Typography;

const Footer: FC = () => {
  const t = useTranslations('Footer');

  return (
    <AntFooter className="bg-white border-t py-6">
      <div className="container mx-auto px-4 text-center">
        <Text className="text-gray-600">{t('copyright')}</Text>
        <div className="mt-2 space-x-4">
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            {t('privacy')}
          </a>
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            {t('terms')}
          </a>
          <a href="/" className="text-gray-500 hover:text-primary text-sm">
            {t('contact')}
          </a>
        </div>
      </div>
    </AntFooter>
  );
};

export default Footer;
