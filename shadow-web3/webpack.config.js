const path = require('path');
const HtmlWebpackPlugin = require('html-webpack-plugin');

module.exports = {
  entry: {
    main: './src/index.ts'
  },
  module: {
    rules: [
      {
        test: /\.tsx?$/,
        use: 'ts-loader',
        exclude: /node_modules/,
      },
      {
        test: /\.css$/i,
        use: ['style-loader', 'css-loader'],
      },
    ],
  },
  resolve: {
    extensions: ['.tsx', '.ts', '.js'],
    fallback: {
      "buffer": require.resolve("buffer/"),
      "crypto": false,
      "fs": false,
      "path": false,
      "os": false
    }
  },
  output: {
    filename: '[name].bundle.js',
    path: path.resolve(__dirname, 'dist'),
    clean: true,
    library: {
      name: 'ShadowyWeb3',
      type: 'umd'
    }
  },
  plugins: [
    new HtmlWebpackPlugin({
      template: './demo/index.html',
      filename: 'demo/index.html'
    })
  ],
  devServer: {
    static: [
      {
        directory: path.join(__dirname, 'dist'),
      },
      {
        directory: path.join(__dirname, 'demo'),
        publicPath: '/demo',
      },
      {
        directory: path.join(__dirname, 'wallet'),
        publicPath: '/wallet',
      }
    ],
    compress: true,
    port: 3000,
    hot: true,
    historyApiFallback: false,
    headers: {
      // Remove COOP/COEP headers that can cause issues with local development
    }
  },
  experiments: {
    asyncWebAssembly: true,
  }
};