import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/product.dart';
import '../services/product_service.dart';
import '../services/order_service.dart';
import 'payment_screen.dart';

class OrdersScreen extends StatefulWidget {
  final List<Map<String, int>> items;
  const OrdersScreen({Key? key, required this.items}) : super(key: key);

  @override
  State<OrdersScreen> createState() => _OrdersScreenState();
}

class _OrdersScreenState extends State<OrdersScreen> {
  late final ProductService productService;
  late final OrderService orderService;
  final TextEditingController _addressController = TextEditingController();
  bool isLoading = false;

  @override
  void initState() {
    super.initState();
    productService = Provider.of<ProductService>(context, listen: false);
    orderService = Provider.of<OrderService>(context, listen: false);
  }

  Future<void> _submitOrder() async {
    final address = _addressController.text.trim();
    if (address.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Пожалуйста, введите адрес доставки')),
      );
      return;
    }

    setState(() => isLoading = true);
    try {
      final response = await orderService.createOrder(widget.items, address);
      final orderId = response['id'] as int;
      final orderTotal = (response['total'] as num).toDouble();

      Navigator.push(
        context,
        MaterialPageRoute(
          builder: (_) => PaymentScreen(
            orderId: orderId,
            orderTotal: orderTotal,
          ),
        ),
      );
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка: $e')),
      );
    } finally {
      setState(() => isLoading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Оформление заказа')),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const Text(
                    'Ваш заказ:',
                    style: TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 8),
                  Expanded(
                    child: ListView.builder(
                      itemCount: widget.items.length,
                      itemBuilder: (context, i) {
                        final item = widget.items[i];
                        return FutureBuilder<Product>(
                          future: productService
                              .getProductById(item['product_id']!),
                          builder: (context, snapshot) {
                            if (snapshot.hasData) {
                              return ListTile(
                                title: Text(snapshot.data!.name),
                                subtitle:
                                    Text('Количество: ${item['quantity']}'),
                                trailing: Text(
                                  '${(snapshot.data!.price * item['quantity']!).toStringAsFixed(2)} ₸',
                                ),
                              );
                            }
                            return const ListTile(
                              title: Center(child: CircularProgressIndicator()),
                            );
                          },
                        );
                      },
                    ),
                  ),
                  const SizedBox(height: 16),
                  TextField(
                    controller: _addressController,
                    decoration: const InputDecoration(
                      labelText: 'Адрес доставки',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 16),
                  ElevatedButton(
                    onPressed: _submitOrder,
                    child: const Text('Перейти к оплате'),
                  ),
                ],
              ),
            ),
    );
  }
}
