/**
 * @type import('hardhat/config').HardhatUserConfig
 */
module.exports = {
    solidity: "0.8.6",
    networks: {
        // blockGasLimit and interval may vary for heavy concurrent test deployments
        hardhat: {
            // x10 of default block gas limit
            blockGasLimit: 124500000,
            mining: {
                auto: false,
                interval: 1000
            }
        }
    }
};

