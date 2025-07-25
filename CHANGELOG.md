# Changelog 

## [1.4.0] - 26-07-2025
- Refactor authentication middleware to improve token validation, error handling and email ctx

## [1.3.0] - 20-07-2025
- Add context to request with authorization header in middleware

## [1.2.1] - 09-07-2025
- Refactor token generation to return expiration time and update service interface

## [1.2.0] - 07-07-2025
- Implement token caching with cache manager and enhance middleware for cached token validation

## [1.1.0] - 07-07-2025
- Add sorted set operations to memory and Redis cache implementations

## [1.0.3] - 05-07-2025
- Enhance client IP retrieval in middleware to support CF-Connecting-IP header

## [1.0.2] - 19-06-2025
- Add refresh token type validation in authentication middleware

## [1.0.1] - 19-06-2025
- Reorder application initialization to set up middleware before routes

## [1.0.0] - 18-06-2025
- Initial release with basic user management features.



