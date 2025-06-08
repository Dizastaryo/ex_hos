import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/auth_provider.dart';
import '../services/appointment_service.dart';

class ProfileScreen extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final authProvider = Provider.of<AuthProvider>(context);
    final appointmentService =
        Provider.of<AppointmentService>(context, listen: false);
    final currentUser = authProvider.currentUser;

    // Извлекаем только username и email из currentUser
    final username = currentUser != null ? currentUser['username'] : null;
    final email = currentUser != null ? currentUser['email'] : null;

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: false,
        backgroundColor: Color(0xFF6A0DAD), // основной цвет
        elevation: 0,
        centerTitle: true,
        title: const Text(
          'Профиль',
          style: TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 22,
            color: Colors.white,
            letterSpacing: 1.2,
          ),
        ),
      ),
      body: currentUser == null
          ? Center(
              child: Text(
                'Вы не авторизованы. Войдите в аккаунт.',
                style: TextStyle(fontSize: 16, color: Colors.grey),
              ),
            )
          : SingleChildScrollView(
              padding: const EdgeInsets.all(16.0),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  const SizedBox(height: 16),
                  Text(
                    username ?? 'Логин не указан',
                    style: const TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.bold,
                      color: Colors.black,
                      shadows: [
                        Shadow(
                          color: Colors.black26,
                          offset: Offset(1, 1),
                          blurRadius: 2,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 20),
                  Text(
                    email ?? 'Email не указан',
                    style: const TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.bold,
                      color: Colors.black,
                      shadows: [
                        Shadow(
                          color: Colors.black26,
                          offset: Offset(1, 1),
                          blurRadius: 2,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 20),
                  // Новая секция: Характеристика
                  ListTile(
                    leading: Icon(Icons.insert_chart_outlined,
                        color: Color(0xFF6A0DAD)),
                    title: Text(
                      'Характеристика',
                      style: TextStyle(fontSize: 16, color: Colors.black),
                    ),
                    trailing: Icon(Icons.arrow_forward_ios, size: 16),
                    onTap: () =>
                        _showCharacteristicsDialog(context, appointmentService),
                  ),
                  const Divider(),
                  ListTile(
                    leading: Icon(Icons.privacy_tip_outlined,
                        color: Color(0xFF6A0DAD)), // основной цвет
                    title: Text(
                      'Политика конфиденциальности',
                      style: TextStyle(fontSize: 16, color: Colors.black),
                    ),
                    trailing: Icon(Icons.arrow_forward_ios, size: 16),
                    onTap: () {
                      Navigator.pushNamed(context, '/privacy_policy');
                    },
                  ),
                  const Divider(),
                  ListTile(
                    leading: Icon(Icons.gavel_outlined,
                        color: Color(0xFF6A0DAD)), // основной цвет
                    title: Text(
                      'Пользовательское соглашение',
                      style: TextStyle(fontSize: 16, color: Colors.black),
                    ),
                    trailing: Icon(Icons.arrow_forward_ios, size: 16),
                    onTap: () {
                      Navigator.pushNamed(context, '/user_agreement');
                    },
                  ),
                  const Divider(),
                  ListTile(
                    leading: Icon(Icons.support_agent,
                        color: Color(0xFF6A0DAD)), // основной цвет
                    title: Text(
                      'Связаться с нами',
                      style: TextStyle(fontSize: 16, color: Colors.black),
                    ),
                    trailing: Icon(Icons.arrow_forward_ios, size: 16),
                    onTap: () {
                      Navigator.pushNamed(context, '/support');
                    },
                  ),
                  const Divider(),
                  const SizedBox(height: 20),
                  ElevatedButton.icon(
                    onPressed: () async {
                      await authProvider.logout(context);
                      Navigator.pushReplacementNamed(context, '/auth');
                    },
                    icon: const Icon(Icons.logout, color: Colors.white),
                    label: const Text(
                      'Выйти из аккаунта',
                      style: TextStyle(color: Colors.white),
                    ),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: Color(0xFF6A0DAD), // основной цвет
                      padding: const EdgeInsets.symmetric(
                          horizontal: 30, vertical: 15),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10),
                      ),
                    ),
                  ),
                ],
              ),
            ),
    );
  }

  void _showCharacteristicsDialog(
      BuildContext context, AppointmentService service) {
    final _formKey = GlobalKey<FormState>();
    String? gender;
    final heightController = TextEditingController();
    final weightController = TextEditingController();
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('Характеристика'),
        content: Form(
          key: _formKey,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                decoration: InputDecoration(labelText: 'Пол'),
                items: ['male', 'female']
                    .map((g) => DropdownMenuItem(value: g, child: Text(g)))
                    .toList(),
                onChanged: (v) => gender = v,
                validator: (v) => v == null ? 'Выберите пол' : null,
              ),
              TextFormField(
                controller: heightController,
                decoration: InputDecoration(labelText: 'Рост (см)'),
                keyboardType: TextInputType.number,
                validator: (v) =>
                    v == null || v.isEmpty ? 'Укажите рост' : null,
              ),
              TextFormField(
                controller: weightController,
                decoration: InputDecoration(labelText: 'Вес (кг)'),
                keyboardType: TextInputType.number,
                validator: (v) => v == null || v.isEmpty ? 'Укажите вес' : null,
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(),
            child: Text('Отмена'),
          ),
          ElevatedButton(
            onPressed: () async {
              if (_formKey.currentState!.validate()) {
                Navigator.of(ctx).pop();
                try {
                  await service.setUserCharacteristics(
                    gender: gender!,
                    height: int.parse(heightController.text),
                    weight: int.parse(weightController.text),
                  );
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('Характеристики сохранены')),
                  );
                } catch (e) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('Ошибка: $e')),
                  );
                }
              }
            },
            child: Text('Сохранить'),
          ),
        ],
      ),
    );
  }
}
