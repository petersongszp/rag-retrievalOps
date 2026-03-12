const withNextIntl = require('next-intl/plugin')();

module.exports = withNextIntl({
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8888/api/:path*',
      },
    ];
  },
});
