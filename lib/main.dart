import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:dio/dio.dart';
import 'package:cookie_jar/cookie_jar.dart';
import 'package:dio_cookie_manager/dio_cookie_manager.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:permission_handler/permission_handler.dart';
import 'package:workmanager/workmanager.dart';

// Провайдеры
import 'providers/auth_provider.dart';
import 'services/product_service.dart';

import 'screens/home_screen.dart';
import 'screens/splash_screen.dart';
import 'screens/auth_screen.dart';
import 'screens/my_cart_screen.dart';
import 'screens/notifications_screen.dart';
import 'screens/reset_password_screen.dart';
import 'screens/admin_home_screen.dart';
import 'screens/moderator_home_screen.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  await initNotifications();
  await requestNotificationPermissions();
  await Workmanager().initialize(callbackDispatcher);

  Workmanager().registerPeriodicTask(
    'notify_rentals_task',
    'notify_rentals',
    frequency: const Duration(days: 1),
    initialDelay: const Duration(seconds: 10),
  );

  final dio = Dio();
  final cookieJar = CookieJar();
  dio.interceptors.add(CookieManager(cookieJar));

  // ✅ Создаём ОДИН экземпляр AuthProvider
  final authProvider = AuthProvider(dio);

  // ✅ Добавляем интерцептор, использующий authProvider
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) async {
        final token = authProvider.token;
        if (token != null && token.isNotEmpty) {
          options.headers['Authorization'] = 'Bearer $token';
        }
        return handler.next(options);
      },
      onError: (error, handler) async {
        if (error.response?.statusCode == 401 &&
            !error.requestOptions.extra.containsKey('retry')) {
          try {
            await authProvider.refreshToken();
            final newToken = authProvider.token;
            if (newToken != null) {
              final retryRequest = error.requestOptions;
              retryRequest.headers['Authorization'] = 'Bearer $newToken';
              retryRequest.extra['retry'] = true;
              final retryResponse = await dio.fetch(retryRequest);
              return handler.resolve(retryResponse);
            }
          } catch (e) {
            await authProvider.logout();
          }
        }
        return handler.next(error);
      },
    ),
  );

  // ✅ Передаём authProvider в MultiProvider
  runApp(
    MultiProvider(
      providers: [
        Provider<Dio>.value(value: dio),
        Provider<CookieJar>.value(value: cookieJar),
        ChangeNotifierProvider<AuthProvider>.value(value: authProvider),
        Provider<ProductService>(
          create: (_) => ProductService(dio),
        ),
      ],
      child: MyApp(),
    ),
  );
}

final FlutterLocalNotificationsPlugin flutterLocalNotificationsPlugin =
    FlutterLocalNotificationsPlugin();

Future<void> initNotifications() async {
  const AndroidInitializationSettings initializationSettingsAndroid =
      AndroidInitializationSettings(
          'app_icon'); // Замените 'app_icon' на вашу иконку

  const DarwinInitializationSettings initializationSettingsDarwin =
      DarwinInitializationSettings(
    requestAlertPermission: true,
    requestBadgePermission: true,
    requestSoundPermission: true,
  );

  final InitializationSettings initializationSettings = InitializationSettings(
    android: initializationSettingsAndroid,
    iOS: initializationSettingsDarwin,
  );

  await flutterLocalNotificationsPlugin.initialize(
    initializationSettings,
    onDidReceiveNotificationResponse:
        (NotificationResponse notificationResponse) {
      switch (notificationResponse.notificationResponseType) {
        case NotificationResponseType.selectedNotification:
          // Обрабатываем нажатие на уведомление
          print('Notification payload: ${notificationResponse.payload}');
          // Навигация на экран уведомлений или выполнение других действий
          runApp(MaterialApp(
            home: NotificationDetailScreen(
                payload: notificationResponse.payload!),
          ));
          break;
        case NotificationResponseType.selectedNotificationAction:
          // Обрабатываем действия с уведомлением (если нужно)
          break;
      }
    },
  );
}

Future<void> requestNotificationPermissions() async {
  // Проверяем, если разрешение уже предоставлено
  PermissionStatus status = await Permission.notification.status;

  // Если разрешение уже предоставлено, ничего не делаем
  if (status.isGranted) {
    print("Разрешение на уведомления уже получено");
    return;
  }

  // Если разрешение отклонено, запрашиваем его
  if (status.isDenied) {
    PermissionStatus newStatus = await Permission.notification.request();
    if (newStatus.isGranted) {
      print("Разрешение на уведомления получено");
    } else {
      print("Разрешение на уведомления отклонено");
    }
  }
  // Если разрешение отклонено навсегда, направляем пользователя в настройки
  if (status.isPermanentlyDenied) {
    print(
        "Разрешение на уведомления отклонено навсегда, переходим в настройки");
    openAppSettings();
  }
}

Future<void> showRentalNotification(String boxNumber) async {
  const AndroidNotificationDetails androidDetails = AndroidNotificationDetails(
    'your_channel_id',
    'your_channel_name',
    channelDescription: 'Напоминания о аренде бокса',
    importance: Importance.max,
    priority: Priority.high,
    playSound: true,
  );

  const DarwinNotificationDetails darwinDetails = DarwinNotificationDetails(
    sound: 'default',
  );

  const NotificationDetails platformDetails = NotificationDetails(
    android: androidDetails,
    iOS: darwinDetails,
  );

  await flutterLocalNotificationsPlugin.show(
    0, // ID уведомления
    'Аренда $boxNumber заканчивается через 5 дней',
    'Пожалуйста, продлите аренду, чтобы избежать потери бокса.',
    platformDetails,
    payload: boxNumber, // Добавляем payload для использования при клике
  );
}

void callbackDispatcher() {
  Workmanager().executeTask((task, inputData) async {
    if (task == 'notify_rentals') {
      // Отправляем уведомления для всех арендуемых боксов
      await showRentalNotification('Бокс #101');
      await showRentalNotification('Бокс #102');
      return Future.value(true);
    }
    return Future.value(false);
  });
}

class MyApp extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Склад для хранения вещей',
      theme: ThemeData(
        primaryColor: const Color(0xFF6C9942),
        colorScheme: const ColorScheme.light(
            primary: Color(0xFF6C9942), secondary: Color(0xFF4A6E2B)),
        fontFamily: 'Montserrat',
        useMaterial3: true,
        appBarTheme: const AppBarTheme(
          backgroundColor: Color(0xFF6C9942),
          foregroundColor: Colors.white,
          elevation: 5,
          centerTitle: true,
        ),
      ),
      initialRoute: '/splash',
      onGenerateRoute: (RouteSettings settings) {
        switch (settings.name) {
          case '/splash':
            return ScalePageRoute(page: SplashScreen());
          case '/auth':
            return ScalePageRoute(page: const AuthScreen());
          case '/main':
            return ScalePageRoute(page: const HomeScreen());
          case '/box-selection':
            return ScalePageRoute(page: const NotificationsScreen());
          case '/my-cart':
            return ScalePageRoute(page: MyCartScreen());
          case '/my-resetpas':
            return ScalePageRoute(page: ResetPasswordScreen());
          case '/admin-home':
            return ScalePageRoute(
                page: const AdminHomeScreen()); // Экран для администратора
          case '/moderator-home':
            return ScalePageRoute(page: const ModeratorHomeScreen());
          default:
            return null;
        }
      },
    );
  }
}

class ScalePageRoute extends PageRouteBuilder {
  final Widget page;

  ScalePageRoute({required this.page})
      : super(
          pageBuilder: (context, animation, secondaryAnimation) => page,
          transitionsBuilder: (context, animation, secondaryAnimation, child) {
            var begin = 0.0; // Начальный масштаб (в 0 раз)
            var end = 1.0; // Конечный масштаб (нормальный размер)
            var curve = Curves.easeInOut;
            var tween = Tween<double>(begin: begin, end: end)
                .chain(CurveTween(curve: curve));
            var scaleAnimation = animation.drive(tween);

            return ScaleTransition(
              scale: scaleAnimation, // Применяем анимацию масштаба
              child: child,
            );
          },
        );
}

class NotificationDetailScreen extends StatelessWidget {
  final String payload;

  const NotificationDetailScreen({super.key, required this.payload});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Детали уведомления'),
      ),
      body: Center(
        child: Text('Подробности для уведомления $payload'),
      ),
    );
  }
}
