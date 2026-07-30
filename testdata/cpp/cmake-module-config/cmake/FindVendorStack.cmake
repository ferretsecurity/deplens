find_package(VendorSDK 3.4 EXACT CONFIG REQUIRED
  NAMES VendorSDK vendor-sdk
  CONFIGS VendorSDKConfig.cmake
  HINTS "${CMAKE_CURRENT_LIST_DIR}/../prefix"
  PATH_SUFFIXES lib/cmake/VendorSDK
  NO_DEFAULT_PATH)
