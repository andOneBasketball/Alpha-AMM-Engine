## 使用方式

### db
go run generate.go

### solc
solc --abi --bin -o ./contracts/build/ ./contracts/IUniswapV2ERC20.sol --overwrite
abigen --bin=./contracts/build/IUniswapV2ERC20.bin --abi=./contracts/build/IUniswapV2ERC20.abi --pkg=IUniswapV2ERC20 --out=./build/IUniswapV2ERC20.go

./tool/solc.exe --abi --bin --base-path ./contracts/  --include-path ./contracts/node_modules -o ./build/ ./contracts/PaymentGw.sol --overwrite
./tool/abigen.exe --bin=./build/PaymentGw.bin --abi=./build/PaymentGw.abi --pkg=PaymentGw --out=./build/payment_gw.go