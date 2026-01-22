# Grabbi Backend API

Golang backend API for Grabbi Store e-commerce platform.

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher

## Setup

1. Install dependencies:
```bash
go mod download
```

2. Create PostgreSQL database:
```bash
createdb grabbi_store
```

3. Copy `.env.example` to `.env` and update with your database credentials:
```bash
cp .env.example .env
```

4. Update `.env` with your database connection details:
```
DATABASE_URL=host=localhost user=postgres password=yourpassword dbname=grabbi_store port=5432 sslmode=disable
```

5. Run the server:
```bash
go run main.go
```

The server will start on port 8080 by default.

## API Endpoints

### Authentication
- `POST /api/auth/register` - Register new user
- `POST /api/auth/login` - Login user
- `GET /api/auth/profile` - Get user profile (protected)

### Products
- `GET /api/products` - Get all products
- `GET /api/products/:id` - Get product by ID
- `POST /api/admin/products` - Create product (admin)
- `PUT /api/admin/products/:id` - Update product (admin)
- `DELETE /api/admin/products/:id` - Delete product (admin)

### Categories
- `GET /api/categories` - Get all categories
- `GET /api/categories/:id` - Get category by ID
- `POST /api/admin/categories` - Create category (admin)
- `PUT /api/admin/categories/:id` - Update category (admin)
- `DELETE /api/admin/categories/:id` - Delete category (admin)

### Cart
- `GET /api/cart` - Get user cart (protected)
- `POST /api/cart` - Add item to cart (protected)
- `PUT /api/cart/:id` - Update cart item (protected)
- `DELETE /api/cart/:id` - Remove item from cart (protected)
- `DELETE /api/cart` - Clear cart (protected)

### Orders
- `POST /api/orders` - Create order (protected)
- `GET /api/orders` - Get user orders (protected)
- `GET /api/orders/:id` - Get order by ID (protected)
- `PUT /api/admin/orders/:id/status` - Update order status (admin)

### Promotions
- `GET /api/promotions` - Get active promotions
- `POST /api/admin/promotions` - Create promotion (admin)
- `PUT /api/admin/promotions/:id` - Update promotion (admin)
- `DELETE /api/admin/promotions/:id` - Delete promotion (admin)

## Default Admin Credentials

- Email: admin@grabbi.com
- Password: admin123

Change these in production!
