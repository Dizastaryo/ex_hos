import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/product.dart';
import '../services/product_service.dart';
import '../services/order_service.dart';
import 'payment_screen.dart';

class ProductDetailScreen extends StatelessWidget {
  final int productId;

  const ProductDetailScreen({super.key, required this.productId});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: FutureBuilder<Product>(
        future: Provider.of<ProductService>(context, listen: false)
            .getProductById(productId),
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }

          if (snapshot.hasError) {
            return Center(child: Text('Ошибка: ${snapshot.error}'));
          }

          if (!snapshot.hasData) {
            return const Center(child: Text('Продукт не найден'));
          }

          final product = snapshot.data!;
          return _buildProductDetails(context, product);
        },
      ),
    );
  }

  Widget _buildProductDetails(BuildContext context, Product product) {
    return Stack(
      children: [
        CustomScrollView(
          slivers: [
            SliverAppBar(
              expandedHeight: 320,
              pinned: true,
              elevation: 2,
              backgroundColor: Colors.white,
              flexibleSpace: FlexibleSpaceBar(
                background: _buildImageGallery(product),
              ),
              leading: IconButton(
                icon: const Icon(Icons.arrow_back, color: Colors.black),
                onPressed: () => Navigator.pop(context),
              ),
            ),
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.all(20),
                child: Container(
                  decoration: BoxDecoration(
                    color: Colors.white,
                    borderRadius: BorderRadius.circular(16),
                    boxShadow: const [
                      BoxShadow(
                        color: Colors.black12,
                        blurRadius: 12,
                        offset: Offset(0, 4),
                      ),
                    ],
                  ),
                  padding: const EdgeInsets.all(20),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        product.name,
                        style: const TextStyle(
                          fontSize: 26,
                          fontWeight: FontWeight.bold,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        '${product.price.toStringAsFixed(2)} ₸',
                        style: const TextStyle(
                          fontSize: 22,
                          fontWeight: FontWeight.w600,
                          color: Colors.green,
                        ),
                      ),
                      const Divider(height: 32),
                      const Text(
                        'Описание',
                        style: TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w500,
                        ),
                      ),
                      const SizedBox(height: 8),
                      Text(
                        product.description,
                        style: const TextStyle(
                          fontSize: 16,
                          height: 1.5,
                          color: Colors.black87,
                        ),
                      ),
                      const SizedBox(height: 100),
                    ],
                  ),
                ),
              ),
            ),
          ],
        ),
        _buildBottomActionBar(context, product),
      ],
    );
  }

  Widget _buildImageGallery(Product product) {
    if (product.imageUrls.isEmpty) {
      return const Center(child: Text('Нет изображений'));
    }

    return PageView.builder(
      itemCount: product.imageUrls.length,
      itemBuilder: (context, index) => ClipRRect(
        borderRadius: const BorderRadius.only(
          bottomLeft: Radius.circular(32),
          bottomRight: Radius.circular(32),
        ),
        child: Image.network(
          'http://172.20.10.2:8000${product.imageUrls[index]}',
          fit: BoxFit.cover,
          loadingBuilder: (context, child, progress) {
            if (progress == null) return child;
            return const Center(child: CircularProgressIndicator());
          },
        ),
      ),
    );
  }

  Widget _buildBottomActionBar(BuildContext context, Product product) {
    return Positioned(
      bottom: 0,
      left: 0,
      right: 0,
      child: SafeArea(
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          decoration: BoxDecoration(
            color: Colors.white,
            boxShadow: const [
              BoxShadow(
                color: Colors.black12,
                blurRadius: 10,
                offset: Offset(0, -2),
              ),
            ],
          ),
          child: Row(
            children: [
              Expanded(
                child: ElevatedButton.icon(
                  icon: const Icon(Icons.shopping_cart),
                  label: const Text('В корзину'),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: Colors.deepOrange,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  onPressed: () async {
                    try {
                      await Provider.of<ProductService>(context, listen: false)
                          .addToCart(product.id);
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('Добавлено в корзину')),
                      );
                      Navigator.pushNamed(context, '/my-cart');
                    } catch (e) {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(content: Text('Ошибка при добавлении: $e')),
                      );
                    }
                  },
                ),
              ),
              const SizedBox(width: 12),
              ElevatedButton(
                child: const Text('Купить', style: TextStyle(fontSize: 16)),
                style: ElevatedButton.styleFrom(
                  backgroundColor: Colors.green,
                  foregroundColor: Colors.white,
                  padding:
                      const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12),
                  ),
                ),
                onPressed: () async {
                  try {
                    final orderService =
                        Provider.of<OrderService>(context, listen: false);
                    final response = await orderService.createOrder(
                      [
                        {'product_id': product.id, 'quantity': 1}
                      ],
                      'Адрес доставки',
                    );

                    final total = (response['total'] as num).toDouble();

                    Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (_) => PaymentScreen(
                          orderId: response['id'] as int,
                          orderTotal: total,
                        ),
                      ),
                    );
                  } catch (e) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('Ошибка создания заказа: $e')),
                    );
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}
