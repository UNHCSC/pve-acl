import MiniCssExtractPlugin from "mini-css-extract-plugin";
import TerserPlugin from "terser-webpack-plugin";
import CssMinimizerPlugin from "css-minimizer-webpack-plugin";
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const entriesDirectory = path.resolve(__dirname, "src/entries");

function getEntries() {
    const files = fs.readdirSync(entriesDirectory, { withFileTypes: true });

    return files
        .filter((file) => file.isFile() && file.name.endsWith(".js"))
        .reduce((entryMap, file) => {
            entryMap[path.parse(file.name).name] = path.join(entriesDirectory, file.name);
            return entryMap;
        }, {});
}

export default function (env = Object.create(null), argv = Object.create(null)) {
    const mode = argv.mode || env.mode || "development";
    const isProduction = mode === "production";

    return {
        mode: mode,
        bail: false,
        watch: !isProduction,
        devtool: isProduction ? false : "source-map",

        watchOptions: {
            aggregateTimeout: 200,
            ignored: /node_modules/
        },

        optimization: {
            minimize: isProduction,
            minimizer: [
                new TerserPlugin({
                    terserOptions: {
                        ecma: 2020,
                        compress: {
                            defaults: true,
                            passes: 3,
                            drop_debugger: true,
                            pure_getters: true,
                            toplevel: true
                        },
                        mangle: {
                            toplevel: true
                        },
                        format: {
                            comments: false,
                            ascii_only: true
                        },
                        keep_classnames: false,
                        keep_fnames: false
                    },
                    extractComments: false
                }),
                new CssMinimizerPlugin()
            ],
            sideEffects: true,
            providedExports: true,
            usedExports: true,
            innerGraph: true,
            concatenateModules: true,
            mangleExports: "size",
            splitChunks: false,
            runtimeChunk: false
        },

        plugins: [
            new MiniCssExtractPlugin({
                filename: "[name].css"
            })
        ],

        entry: getEntries(),

        resolve: {
            extensions: [".js"]
        },

        module: {
            rules: [{
                test: /\.css$/i,
                use: [MiniCssExtractPlugin.loader, "css-loader", "postcss-loader"]
            }]
        },

        output: {
            filename: "[name].js",
            path: path.resolve(__dirname, "static/build"),
            clean: true
        }
    };
};
