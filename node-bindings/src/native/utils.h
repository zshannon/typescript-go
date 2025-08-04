#ifndef UTILS_H
#define UTILS_H

#include <string>
#include <cstdlib>
#include <cstring>

// Utility functions for string manipulation
inline char* duplicate_string(const std::string& str) {
    char* result = static_cast<char*>(malloc(str.length() + 1));
    if (result) {
        strcpy(result, str.c_str());
    }
    return result;
}

#endif // UTILS_H