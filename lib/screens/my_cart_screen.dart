// lib/screens/my_cart_screen.dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
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
      setState(() {
        _cartItems = (response['items'] as List).cast<Map<String, dynamic>>();
        _totalPrice = (response['total_price'] as num).toDouble();
      });
    } catch (_) {
      setState(() {
        _cartItems = [];
        _totalPrice = 0.0;
      });
    } finally {
      setState(() => _isLoading = false);
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

  Future<void> _updateQuantity(int productId, int newQuantity) async {
    if (newQuantity < 1) return;
    await _productService.updateCart(productId, newQuantity);
    await _loadCart();
  }

  void _navigateToCheckout(List<Map<String, int>> items) {
    Navigator.push(
      context,
      MaterialPageRoute(builder: (_) => OrdersScreen(items: items)),
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
            onPressed: _cartItems.isEmpty ? null : _clearCart,
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
                          return _buildCartItem(_cartItems[index]);
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
          const Icon(
            Icons.shopping_cart_outlined,
            size: 80,
            color: Colors.grey,
          ),
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
              MaterialPageRoute(builder: (_) => const ProductsScreen()),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCartItem(Map<String, dynamic> item) {
    final productDataNullable = item['product'] as Map<String, dynamic>?;
    final quantity = item['quantity'] as int?;

    if (productDataNullable == null || quantity == null) {
      return const ListTile(
        title: Text('Некорректные данные товара'),
        leading: Icon(Icons.error_outline, color: Colors.red),
      );
    }

    final pd = productDataNullable;
    final productId = pd['id'] as int?;
    if (productId == null) {
      return const ListTile(
        title: Text('Некорректные данные товара'),
        leading: Icon(Icons.error_outline, color: Colors.red),
      );
    }

    final name = pd['name'] as String? ?? 'Неизвестный продукт';
    final price = (pd['price'] as num?)?.toDouble() ?? 0.0;
    final images = (pd['images'] as List).cast<Map<String, dynamic>>();
    final imageUrl = images.isNotEmpty
        ? 'http://172.20.10.2:8000${images.first['image_url']}'
        : 'https://via.placeholder.com/150';

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      leading: ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Image.network(
          imageUrl,
          width: 60,
          height: 60,
          fit: BoxFit.cover,
        ),
      ),
      title: Text(name),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('${price.toStringAsFixed(2)} ₸ x $quantity'),
          Text(
            'Итого: ${(price * quantity).toStringAsFixed(2)} ₸',
            style: const TextStyle(fontWeight: FontWeight.bold),
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              IconButton(
                icon: const Icon(Icons.remove_circle_outline),
                onPressed: () => _updateQuantity(productId, quantity - 1),
              ),
              Text(quantity.toString()),
              IconButton(
                icon: const Icon(Icons.add_circle_outline),
                onPressed: () => _updateQuantity(productId, quantity + 1),
              ),
            ],
          ),
        ],
      ),
      trailing: Column(
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

  Widget _buildTotalSection() {
    final items = _cartItems.map((item) {
      final prod = item['product'] as Map<String, dynamic>;
      return {
        'product_id': prod['id'] as int,
        'quantity': item['quantity'] as int
      };
    }).toList();

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
              onPressed: () => _navigateToCheckout(items),
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
