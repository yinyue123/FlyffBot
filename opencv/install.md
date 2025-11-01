# OpenCV Installation Guide

## Prerequisites
- Python 3.7 or higher
- pip (Python package installer)

## Installation Methods

### Method 1: Using pip (Recommended)

Install OpenCV with Python bindings:

```bash
pip install opencv-python
```

For the full package with additional modules:

```bash
pip install opencv-contrib-python
```

### Method 2: Using conda

If you're using Anaconda or Miniconda:

```bash
conda install -c conda-forge opencv
```

### Method 3: Install with NumPy

OpenCV requires NumPy. Install both together:

```bash
pip install numpy opencv-python
```

## Verification

After installation, verify OpenCV is working:

```python
import cv2
print(cv2.__version__)
```

## Additional Dependencies

For this game character recognition project, you may also need:

```bash
pip install numpy opencv-python
```

## Platform-Specific Notes

### macOS
- No additional steps required
- Works with both Intel and Apple Silicon (M1/M2) processors

### Windows
- Make sure Microsoft Visual C++ Redistributable is installed
- Download from: https://aka.ms/vs/16/release/vc_redist.x64.exe

### Linux (Ubuntu/Debian)
```bash
sudo apt-get update
sudo apt-get install python3-opencv
```

Or use pip:
```bash
pip install opencv-python
```

## Troubleshooting

If you encounter import errors:
1. Make sure you're using the correct Python environment
2. Try reinstalling: `pip uninstall opencv-python && pip install opencv-python`
3. Check Python version compatibility: `python --version`

## Quick Start Test

Create a test file to verify installation:

```python
import cv2
import numpy as np

# Create a simple test image
img = np.zeros((100, 100, 3), dtype=np.uint8)
cv2.imshow('Test', img)
cv2.waitKey(0)
cv2.destroyAllWindows()
print("OpenCV is working!")
```
