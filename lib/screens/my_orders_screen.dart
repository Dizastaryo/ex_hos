import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/order_service.dart';

class MyOrdersScreen extends StatefulWidget {
  const MyOrdersScreen({Key? key}) : super(key: key);

  @override
  State<MyOrdersScreen> createState() => _MyOrdersScreenState();
}

class _MyOrdersScreenState extends State<MyOrdersScreen> {
  late final OrderService orderService;
  bool isLoading = true;
  List<dynamic> orders = [];

  @override
  void initState() {
    super.initState();
    orderService = Provider.of<OrderService>(context, listen: false);
    _fetchOrders();
  }

  Future<void> _fetchOrders() async {
    try {
      final fetchedOrders = await orderService.getMyOrders();
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
            title: Text('Заказ #$orderId'),
            content: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: const [
                      Icon(Icons.location_on_outlined, size: 20),
                      SizedBox(width: 6),
                      Text('Адрес доставки:',
                          style: TextStyle(fontWeight: FontWeight.bold)),
                    ],
                  ),
                  Text(orderDetails['shipping_address'] ?? 'Нет данных'),
                  const SizedBox(height: 12),
                  Row(
                    children: const [
                      Icon(Icons.check_circle_outline, size: 20),
                      SizedBox(width: 6),
                      Text('Статус заказа:',
                          style: TextStyle(fontWeight: FontWeight.bold)),
                    ],
                  ),
                  Text(orderDetails['status'] ?? 'Неизвестно'),
                  const SizedBox(height: 12),
                  const Text('Состав заказа:',
                      style: TextStyle(fontWeight: FontWeight.bold)),
                  const SizedBox(height: 6),
                  ...orderDetails['items'].map<Widget>((item) {
                    return Card(
                      elevation: 2,
                      margin: const EdgeInsets.symmetric(vertical: 4),
                      shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8)),
                      child: ListTile(
                        contentPadding: const EdgeInsets.all(8),
                        leading: const Icon(Icons.shopping_bag_outlined),
                        title: Text('Товар ID: ${item['product_id']}'),
                        subtitle: Text('Количество: ${item['quantity']}'),
                      ),
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        title: const Text('Мои заказы'),
      ),
      body: isLoading
          ? const Center(child: CircularProgressIndicator())
          : orders.isEmpty
              ? const Center(child: Text('У вас нет заказов'))
              : ListView.builder(
                  itemCount: orders.length,
                  itemBuilder: (context, index) {
                    final order = orders[index];
                    return Card(
                      margin: const EdgeInsets.all(8),
                      child: ListTile(
                        title: Text('Заказ #${order['id']}'),
                        subtitle: Text('Статус: ${order['status']}'),
                        trailing: IconButton(
                          icon: const Icon(Icons.info_outline),
                          onPressed: () => _showOrderDetails(order['id']),
                        ),
                      ),
                    );
                  },
                ),
    );
  }
}
