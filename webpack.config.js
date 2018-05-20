const path = require('path');
const webpack = require('webpack');
const uglify = require('uglifyjs-webpack-plugin');

module.exports = {
    entry: path.join(__dirname, 'typescript/Zerodrop.ts'),
    output: {
        filename: 'zerodrop.js',
        path: path.join(__dirname, 'static')
    },
    module: {
        rules: [
            {
                test: /\.tsx?$/,
                loader: 'ts-loader',
                exclude: /node_modules/
            }
        ]
    },
    resolve: {
        extensions: ['.tsx', '.ts', '.js']
    },
    devtool: 'source-map',
    plugins: [
        new webpack.ProvidePlugin({
            $: 'jquery',
            jQuery: 'jquery',
            Popper: 'popper.js'
        }),
        new uglify({
            sourceMap: true
        })
    ]
}
