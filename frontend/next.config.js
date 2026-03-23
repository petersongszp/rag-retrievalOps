module.exports = {
  async rewrites() {
    return [
      {
        source: '/api/:path*',
        destination: 'http://localhost:8899/api/:path*',
      },
    ];
  },
};
