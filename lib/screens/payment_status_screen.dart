import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/order_service.dart';

class PaymentStatusScreen extends StatefulWidget {
  final int orderId;
  const PaymentStatusScreen({Key? key, required this.orderId})
      : super(key: key);

  @override
  State<PaymentStatusScreen> createState() => _PaymentStatusScreenState();
}

class _PaymentStatusScreenState extends State<PaymentStatusScreen> {
  String paymentStatus = "Загрузка...";
  bool isLoading = false;

  Future<void> _getPaymentStatus() async {
    setState(() => isLoading = true);
    try {
      final response = await Provider.of<OrderService>(context, listen: false)
          .getPaymentStatus(widget.orderId);
      setState(() {
        paymentStatus = response['status'];
      });
    } catch (e) {
      setState(() {
        paymentStatus = "Ошибка получения статуса";
      });
    } finally {
      setState(() => isLoading = false);
    }
  }

  @override
  void initState() {
    super.initState();
    _getPaymentStatus();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Статус платежа')),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            const Icon(Icons.credit_card, size: 80, color: Colors.blue),
            const SizedBox(height: 24),
            Text(
              'Статус платежа:',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 16),
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: _getStatusColor(),
                borderRadius: BorderRadius.circular(12),
              ),
              child: Text(
                paymentStatus,
                style: const TextStyle(
                  fontSize: 20,
                  color: Colors.white,
                  fontWeight: FontWeight.bold,
                ),
              ),
            ),
            const SizedBox(height: 32),
            ElevatedButton.icon(
              icon: const Icon(Icons.refresh),
              label: const Text('Обновить статус'),
              onPressed: isLoading ? null : _getPaymentStatus,
            ),
            const SizedBox(height: 16),
            TextButton(
              onPressed: () => Navigator.pushNamedAndRemoveUntil(
                  context, '/', (route) => false),
              child: const Text('Вернуться на главную'),
            ),
          ],
        ),
      ),
    );
  }

  Color _getStatusColor() {
    switch (paymentStatus.toLowerCase()) {
      case 'успешно':
        return Colors.green;
      case 'в обработке':
        return Colors.orange;
      case 'ошибка':
        return Colors.red;
      default:
        return Colors.grey;
    }
  }
}
