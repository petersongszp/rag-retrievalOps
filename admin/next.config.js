module.exports = {
  async rewrites() {
    const apiBase = (
      process.env.API_PROXY_TARGET ||
      process.env.NEXT_PUBLIC_API_BASE_URL ||
      'http://rag-server:8899'
    ).replace(/\/api\/?$/, '').replace(/\/$/, '');

    return [
      {
        source: '/api/v1/auth/:path*',
        destination: `${apiBase}/v1/auth/:path*`,
      },
      {
        source: '/api/v1/api-keys/:path*',
        destination: `${apiBase}/v1/api-keys/:path*`,
      },
      {
        source: '/api/v1/tenant/:path*',
        destination: `${apiBase}/v1/tenant/:path*`,
      },
      {
        source: '/api/admin/:path*',
        destination: `${apiBase}/api/admin/:path*`,
      },
      {
        source: '/api/kb/:path*',
        destination: `${apiBase}/api/kb/:path*`,
      },
      {
        source: '/api/:path*',
        destination: `${apiBase}/api/:path*`,
      },
    ];
  },
};
