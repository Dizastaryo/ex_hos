// lib/screens/admin_orders_screen.dart
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/order_service.dart';

class AdminOrdersScreen extends StatefulWidget {
  const AdminOrdersScreen({Key? key}) : super(key: key);

  @override
  State<AdminOrdersScreen> createState() => _AdminOrdersScreenState();
}

class _AdminOrdersScreenState extends State<AdminOrdersScreen> {
  late final OrderService orderService;
  bool isLoading = true;
  List<dynamic> orders = [];

  @override
  void initState() {
    super.initState();
    orderService = Provider.of<OrderService>(context, listen: false);
    _fetchAllOrders();
  }

  Future<void> _fetchAllOrders() async {
    try {
      final fetchedOrders = await orderService.getAllOrders();
      setState(() {
        orders = fetchedOrders;
        isLoading = false;
      });
    } catch (e) {
      setState(() {
        isLoading = false;
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка: $e')),
      );
    }
  }

  Future<void> _showOrderDetails(int orderId) async {
    try {
      final orderDetails = await orderService.getOrderById(orderId);
      showDialog(
        context: context,
        builder: (context) {
          return AlertDialog(
            title: Text('Детали заказа #$orderId'),
            content: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Адрес: ${orderDetails['shipping_address']}'),
                  SizedBox(height: 8),
                  Text('Статус: ${orderDetails['status']}'),
                  SizedBox(height: 8),
                  Text('Товары:'),
                  ...orderDetails['items'].map<Widget>((item) {
                    return Text(
                      'Product ID: ${item['product_id']}, Quantity: ${item['quantity']}',
                    );
                  }).toList(),
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('Закрыть'),
              ),
            ],
          );
        },
      );
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка: $e')),
      );
    }
  }

  Future<void> _cancelOrder(int orderId) async {
    try {
      await orderService.cancelOrder(orderId);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Заказ #$orderId успешно отменён')),
      );
      _fetchAllOrders(); // Обновляем список заказов
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('Ошибка отмены заказа: $e')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Все заказы'),
      ),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : orders.isEmpty
              ? const Center(child: Text('Нет заказов для отображения'))
              : ListView.builder(
                  itemCount: orders.length,
                  itemBuilder: (context, index) {
                    final order = orders[index];
                    return Card(
                      margin: const EdgeInsets.all(8),
                      child: ListTile(
                        title: Text('Заказ #${order['id']}'),
                        subtitle: Text('Статус: ${order['status']}'),
                        trailing: Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            IconButton(
                              icon: const Icon(Icons.info_outline),
                              onPressed: () => _showOrderDetails(order['id']),
                            ),
                            IconButton(
                              icon: const Icon(Icons.cancel_outlined),
                              onPressed: () => _cancelOrder(order['id']),
                            ),
                          ],
                        ),
                      ),
                    );
                  },
                ),
    );
  }
}
