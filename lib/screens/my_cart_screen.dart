// lib/screens/my_cart_screen.dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'dart:convert';
import '../services/product_service.dart';
import '../services/order_service.dart';
import 'products_screen.dart';
import 'orders_screen.dart';

class MyCartScreen extends StatefulWidget {
  const MyCartScreen({super.key});

  @override
  State<MyCartScreen> createState() => _MyCartScreenState();
}

class _MyCartScreenState extends State<MyCartScreen> {
  late ProductService _productService;
  late OrderService _orderService;
  bool _isLoading = true;
  List<Map<String, dynamic>> _cartItems = [];
  double _totalPrice = 0.0;

  @override
  void initState() {
    super.initState();
    _productService = Provider.of<ProductService>(context, listen: false);
    _orderService = Provider.of<OrderService>(context, listen: false);
    _loadCart();
  }

  Future<void> _loadCart() async {
    setState(() => _isLoading = true);
    try {
      final response = await _productService.getCart();
      final items = jsonDecode(response['items']) as List;
      setState(() {
        _cartItems = items.cast<Map<String, dynamic>>();
        _totalPrice = (response['total_price'] as num).toDouble();
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _cartItems = [];
        _totalPrice = 0.0;
        _isLoading = false;
      });
    }
  }

  Future<void> _removeItem(int productId) async {
    await _productService.removeFromCart(productId);
    await _loadCart();
  }

  Future<void> _clearCart() async {
    await _productService.clearCart();
    await _loadCart();
  }

  void _navigateToCheckout(List<Map<String, int>> items) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (context) => OrdersScreen(items: items),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Моя корзина'),
        actions: [
          IconButton(
            icon: const Icon(Icons.delete_forever),
            onPressed: _clearCart,
            tooltip: 'Очистить корзину',
          ),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _cartItems.isEmpty
              ? _buildEmptyCart()
              : Column(
                  children: [
                    Expanded(
                      child: ListView.separated(
                        itemCount: _cartItems.length,
                        separatorBuilder: (_, __) => const Divider(),
                        itemBuilder: (context, index) {
                          final item = _cartItems[index];
                          return _buildCartItem(item);
                        },
                      ),
                    ),
                    _buildTotalSection(),
                  ],
                ),
    );
  }

  Widget _buildEmptyCart() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.shopping_cart_outlined,
              size: 80, color: Colors.grey),
          const SizedBox(height: 20),
          const Text(
            'Ваша корзина пуста',
            style: TextStyle(fontSize: 18, color: Colors.grey),
          ),
          const SizedBox(height: 20),
          ElevatedButton.icon(
            icon: const Icon(Icons.search),
            label: const Text('Перейти к товарам'),
            onPressed: () => Navigator.pushReplacement(
              context,
              MaterialPageRoute(builder: (context) => const ProductsScreen()),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCartItem(Map<String, dynamic> item) {
    final productId = item['product_id'] as int?;
    final quantity = item['quantity'] as int?;

    if (productId == null || quantity == null) {
      return const ListTile(
        title: Text('Некорректные данные товара'),
        leading: Icon(Icons.error_outline, color: Colors.red),
      );
    }

    return FutureBuilder(
      future: _productService.getProductById(productId),
      builder: (context, snapshot) {
        if (snapshot.hasError) {
          return ListTile(
            title: const Text('Ошибка загрузки товара'),
            subtitle: Text('ID: $productId'),
            leading: const Icon(Icons.error, color: Colors.red),
            trailing: IconButton(
              icon: const Icon(Icons.delete_forever),
              onPressed: () => _removeItem(productId),
            ),
          );
        }

        if (snapshot.hasData) {
          final product = snapshot.data!;
          return ListTile(
            contentPadding:
                const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
            leading: product.imageUrls.isNotEmpty
                ? Image.network(
                    'http://172.20.10.2:8000${product.imageUrls.first}',
                    width: 60,
                    height: 60,
                    fit: BoxFit.cover,
                  )
                : const Icon(Icons.shopping_bag, size: 60),
            title: Text(product.name),
            subtitle: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${product.price.toStringAsFixed(2)} ₸ x $quantity'),
                Text(
                  'Итого: ${(product.price * quantity).toStringAsFixed(2)} ₸',
                  style: const TextStyle(fontWeight: FontWeight.bold),
                ),
              ],
            ),
            trailing: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                IconButton(
                  icon: const Icon(Icons.arrow_forward_ios),
                  onPressed: () => _navigateToCheckout([
                    {'product_id': productId, 'quantity': quantity}
                  ]),
                ),
                IconButton(
                  icon: const Icon(Icons.delete_outline),
                  color: Colors.red,
                  onPressed: () => _removeItem(productId),
                ),
              ],
            ),
          );
        }

        return const ListTile(
          leading: CircularProgressIndicator(),
          title: Text('Загрузка информации о товаре...'),
        );
      },
    );
  }

  Widget _buildTotalSection() {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: Colors.grey[100],
        borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
      ),
      child: Column(
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              const Text(
                'Общая сумма:',
                style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
              ),
              Text(
                '${_totalPrice.toStringAsFixed(2)} ₸',
                style: const TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                  color: Colors.green,
                ),
              ),
            ],
          ),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 16),
                backgroundColor: Colors.blue,
              ),
              onPressed: () => _navigateToCheckout(
                _cartItems
                    .map((e) => {
                          'product_id': e['product_id'] as int,
                          'quantity': e['quantity'] as int
                        })
                    .toList(),
              ),
              child: const Text(
                'Оформить весь заказ',
                style: TextStyle(fontSize: 16, color: Colors.white),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
