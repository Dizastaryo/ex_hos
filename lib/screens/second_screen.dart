import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';

class SecondScreen extends StatefulWidget {
  @override
  _SecondScreenState createState() => _SecondScreenState();
}

class _SecondScreenState extends State<SecondScreen> {
  @override
  void initState() {
    super.initState();
    _autoLogin();
  }

  Future<void> _autoLogin() async {
    try {
      await context.read<AuthProvider>().autoLogin(context);
    } catch (e) {
      // Показываем ошибку, если автологин не удался
      showDialog(
        context: context,
        builder: (_) => AlertDialog(
          title: Text("Ошибка"),
          content: Text("Не удалось выполнить автоматический вход: $e"),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text("OK"),
            ),
          ],
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.white,
      appBar: AppBar(
        title: Text(""),
        backgroundColor: Colors.white,
      ),
      body: Center(
        child: CircularProgressIndicator(),
      ),
    );
  }
}
