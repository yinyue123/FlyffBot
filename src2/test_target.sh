#!/bin/bash
# Test script for debug_target.go

# Copy the test image to status.jpeg
cp '/Users/yinyue/Library/Containers/com.tencent.xinWeChat/Data/Library/Application Support/com.tencent.xinWeChat/2.0b4.0.9/98b08a19d30dfcae0cd89385a8fdcb20/Message/MessageTemp/9e20f478899dc29eb19741386f9343c8/Image/12701763044389_.pic.jpg' status.jpeg

# Run the debug program
go run -tags ignore debug_target.go
