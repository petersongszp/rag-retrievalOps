module.exports = {
  async rewrites() {
    const apiBase = (process.env.API_PROXY_TARGET || process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8899/api').replace(
      /\/$/,
      '',
    );
    return [
      {
        source: '/api/:path*',
        destination: `${apiBase}/:path*`,
      },
    ];
  },
};
